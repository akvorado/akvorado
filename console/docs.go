// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"

	highlighting "github.com/yuin/goldmark-highlighting/v3"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

// Media types the documentation can be served as.
const (
	docsTypeMarkdown = "text/markdown"
	docsTypeJSON     = "application/json"
	docsTypeHTML     = "text/html"
)

var (
	internalLinkRegexp = regexp.MustCompile("^(([0-9]+)-([a-z]+).md)(#.*|$)")

	// markdownLinkRegexp matches a link to another document, in the Markdown
	// source this time.
	markdownLinkRegexp = regexp.MustCompile(`\]\(([0-9]+)-([a-z]+)\.md(#[^)]*)?\)`)

	// documentTitleRegexp matches the title of a document.
	documentTitleRegexp = regexp.MustCompile(`(?m)^# +(.*\S)`)

	// docSections gives the Diátaxis section a document belongs to, depending on
	// the number prefixing its file name. The first matching upper bound wins.
	// Documents outside any section (the introduction and the changelog) get an
	// empty string.
	docSections = []struct {
		upTo    int
		section string
	}{
		{0, ""},
		{9, "Tutorials"},
		{49, "How-to guides"},
		{79, "Reference"},
		{98, "Explanation"},
	}

	// renamedDocuments maps the name of a document that does not exist anymore
	// to its replacement, to keep old links working.
	renamedDocuments = map[string]string{
		"operations": "exporters",
	}
)

var (
	// tocParser only looks for the headers of a document.
	tocParser = parser.New(parser.WithAutoHeadingID())

	// docParser and docRenderer turn a document into the HTML served to the
	// web interface.
	docParser = parser.New(
		parser.WithExtensions(
			extension.TableParser,
			extension.TypographerParser,
			highlighting.Parser,
			admonitionParser,
		),
		parser.WithAutoHeadingID(),
		parser.WithASTTransformers(
			util.Prioritized[parser.ASTTransformer](&linkTransformer{}, 500),
		),
	)
	docRenderer = html.New(
		html.WithExtensions(
			extension.TableHTMLRenderer,
			highlighting.NewHTMLRenderer(
				highlighting.WithCustomStyle(draculaStyle),
			),
			admonitionHTMLRenderer,
		),
	)
)

// Header describes a document header.
type Header struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// DocumentTOC describes the TOC of a document
type DocumentTOC struct {
	Name    string   `json:"name"`
	Section string   `json:"section"`
	Headers []Header `json:"headers"`
}

// documentSection returns the section a document belongs to.
func documentSection(number string) string {
	n, err := strconv.Atoi(number)
	if err != nil {
		return ""
	}
	for _, s := range docSections {
		if n <= s.upTo {
			return s.section
		}
	}
	return ""
}

// documentHeaders returns the headers of a document, to build its ToC.
func documentHeaders(markdown []byte) []Header {
	headers := []Header{}
	ast.Walk(tocParser.Parse(markdown), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		heading, ok := n.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		id, ok := heading.Attribute("id")
		lines := heading.Source()
		if !ok || len(lines) == 0 {
			return ast.WalkContinue, nil
		}
		headers = append(headers, Header{
			ID:    id.Value(markdown),
			Level: heading.Level,
			Title: lines[len(lines)-1].Str(markdown),
		})
		return ast.WalkContinue, nil
	})
	return headers
}

func (c *Component) docsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	requestedDocument := req.PathValue("name")
	if replacement, ok := renamedDocuments[requestedDocument]; ok {
		requestedDocument = replacement
	}
	w.Header().Set("Vary", "Accept")

	markdown := c.findDocument(requestedDocument)
	if markdown == nil {
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "Document not found."})
		return
	}
	w.Header().Set("Cache-Control", "max-age=300, public")

	// Unless JSON is preferred, answer with the source: this is the most useful
	// answer for a crawler or an LLM.
	if !prefersOverMarkdown(req.Header.Get("Accept"), docsTypeJSON) {
		writeMarkdownDocument(w, markdown)
		return
	}

	parserContext := parser.NewContext()
	parserContext.Set(documentNameKey, requestedDocument)

	var buf strings.Builder
	node := docParser.Parse(markdown, parser.WithContext(parserContext))
	if err := docRenderer.Render(&buf, markdown, node); err != nil {
		c.r.Err(err).Str("path", requestedDocument).Msg("unable to render markdown document")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to render document."})
		return
	}

	httpserver.WritePureJSON(w, http.StatusOK, helpers.M{
		"markdown": buf.String(),
		"toc":      c.documentTOC(),
	})
}

// documentTOC returns the ToC of each document. It does not depend on the
// request, so it is only built once, unless the files are served from disk.
func (c *Component) documentTOC() []DocumentTOC {
	if c.config.ServeLiveFS {
		return c.buildDocumentTOC()
	}
	c.documentTOCOnce.Do(func() {
		c.documentTOCCache = c.buildDocumentTOC()
	})
	return c.documentTOCCache
}

// buildDocumentTOC returns the ToC of each document of the documentation.
func (c *Component) buildDocumentTOC() []DocumentTOC {
	toc := []DocumentTOC{}
	docs := c.embedOrLiveFS("data/docs")
	entries, err := fs.ReadDir(docs, ".")
	if err != nil {
		c.r.Err(err).Msg("unable to list documentation files")
		return toc
	}
	for _, entry := range entries {
		matches := internalLinkRegexp.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		content, err := fs.ReadFile(docs, entry.Name())
		if err != nil {
			c.r.Err(err).Str("path", entry.Name()).Msg("unable to open documentation file")
			continue
		}
		toc = append(toc, DocumentTOC{
			Name:    matches[3],
			Section: documentSection(matches[2]),
			Headers: documentHeaders(content),
		})
	}
	return toc
}

// findDocument returns the source of a document, or nil when there is no such
// document.
func (c *Component) findDocument(name string) []byte {
	if replacement, ok := renamedDocuments[name]; ok {
		name = replacement
	}
	docs := c.embedOrLiveFS("data/docs")
	entries, err := fs.ReadDir(docs, ".")
	if err != nil {
		c.r.Err(err).Msg("unable to list documentation files")
		return nil
	}
	for _, entry := range entries {
		matches := internalLinkRegexp.FindStringSubmatch(entry.Name())
		if matches == nil || matches[3] != name {
			continue
		}
		content, err := fs.ReadFile(docs, entry.Name())
		if err != nil {
			c.r.Err(err).Str("path", entry.Name()).Msg("unable to open documentation file")
			return nil
		}
		return content
	}
	return nil
}

// documentTitle returns the title of a document, which is its first level-1
// heading.
func (c *Component) documentTitle(docs fs.FS, fileName string) string {
	f, err := docs.Open(fileName)
	if err != nil {
		c.r.Err(err).Str("path", fileName).Msg("unable to open documentation file")
		return ""
	}
	defer f.Close()
	// The title is on the first line, there is no need to read the whole file.
	head, err := io.ReadAll(io.LimitReader(f, 1024))
	if err != nil {
		c.r.Err(err).Str("path", fileName).Msg("unable to read documentation file")
		return ""
	}
	matches := documentTitleRegexp.FindSubmatch(head)
	if matches == nil {
		return ""
	}
	return string(matches[1])
}

// documentIndex builds a Markdown index of the documentation: each document
// with its title, grouped by section. Documents without a title are left out.
// It returns nil on error.
func (c *Component) documentIndex() []byte {
	docs := c.embedOrLiveFS("data/docs")
	entries, err := fs.ReadDir(docs, ".")
	if err != nil {
		c.r.Err(err).Msg("unable to list documentation files")
		return nil
	}
	var index strings.Builder
	index.WriteString("# Akvorado documentation\n\n")
	section := ""
	for _, entry := range entries {
		matches := internalLinkRegexp.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		title := c.documentTitle(docs, entry.Name())
		if title == "" {
			continue
		}
		if current := documentSection(matches[2]); current != section {
			section = current
			if section != "" {
				fmt.Fprintf(&index, "- **%s**\n", section)
			}
		}
		indent := ""
		if section != "" {
			indent = "  "
		}
		fmt.Fprintf(&index, "%s- [%s](%sdocs/%s)\n",
			indent, title, c.urlPrefix(), matches[3])
	}
	return []byte(index.String())
}

// writeMarkdownDocument answers with the source of a document. Only the links
// to the other documents are rewritten, to the matching URLs.
func writeMarkdownDocument(w http.ResponseWriter, markdown []byte) {
	w.Header().Set("Content-Type", fmt.Sprintf("%s; charset=utf-8", docsTypeMarkdown))
	w.Write(markdownLinkRegexp.ReplaceAll(markdown, []byte("](${2}${3})")))
}

// documentFromPath returns the document a URL of the web interface points to.
// An empty name is for the index. The second value is false when the URL is not
// a documentation page.
func documentFromPath(path string) (string, bool) {
	if path == "/docs" {
		return "", true
	}
	name, ok := strings.CutPrefix(path, "/docs/")
	if !ok || strings.Contains(name, "/") {
		return "", false
	}
	return name, true
}

// serveMarkdownDocument answers with the source of a document when the request
// is for a documentation page of the web interface and the client does not
// prefer HTML. A browser asks for HTML, so it gets the application. The
// documentation root gets an index of the documents.
func (c *Component) serveMarkdownDocument(w http.ResponseWriter, r *http.Request) bool {
	name, ok := documentFromPath(r.URL.Path)
	if !ok {
		return false
	}
	// From now on, the answer depends on the Accept header, even when we let
	// the application handle the request.
	w.Header().Set("Vary", "Accept")
	if prefersOverMarkdown(r.Header.Get("Accept"), docsTypeHTML) {
		return false
	}
	var markdown []byte
	if name == "" {
		markdown = c.documentIndex()
	} else {
		markdown = c.findDocument(name)
	}
	if markdown == nil {
		// Let the application display its own error message.
		return false
	}
	w.Header().Set("Cache-Control", "max-age=300, public")
	writeMarkdownDocument(w, markdown)
	return true
}

// mediaTypeQuality returns the quality the Accept header gives to a media type.
// It is 0 when the media type is not named. Wildcards are ignored: a client
// accepting anything is served the default answer.
func mediaTypeQuality(accept, mediaType string) float64 {
	for entry := range strings.SplitSeq(accept, ",") {
		mediaRange, params, err := mime.ParseMediaType(entry)
		if err != nil || mediaRange != mediaType {
			continue
		}
		if quality, err := strconv.ParseFloat(params["q"], 64); err == nil {
			return quality
		}
		return 1
	}
	return 0
}

// prefersOverMarkdown tells if the client asks for the provided media type
// rather than for Markdown, which is the default answer.
func prefersOverMarkdown(accept, mediaType string) bool {
	quality := mediaTypeQuality(accept, mediaType)
	return quality > 0 && quality > mediaTypeQuality(accept, docsTypeMarkdown)
}

// documentNameKey is the parser context key holding the name of the document
// being parsed.
var documentNameKey = parser.NewContextKey()

// linkTransformer rewrites the links to the other documents, the links to an
// anchor and the images to URLs relative to the <base> tag of the web
// interface.
type linkTransformer struct{}

// Transform implements parser.ASTTransformer
func (r *linkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	currentDocument, _ := pc.Get(documentNameKey).(string)
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Link:
			destination := node.Destination.Value(source)
			if matches := internalLinkRegexp.FindStringSubmatch(destination); matches != nil {
				node.Destination = text.NewSingleLineValueFromString(
					fmt.Sprintf("docs/%s%s", matches[3], matches[4]), text.IdentityDecoder)
			} else if strings.HasPrefix(destination, "#") {
				node.Destination = text.NewSingleLineValueFromString(
					fmt.Sprintf("docs/%s%s", currentDocument, destination), text.IdentityDecoder)
			}
		case *ast.Image:
			if path := node.Destination.Value(source); !strings.Contains(path, "/") {
				node.Destination = text.NewSingleLineValueFromString(
					fmt.Sprintf("assets/docs/%s", path), text.IdentityDecoder)
			}
		}
		return ast.WalkContinue, nil
	})
}
