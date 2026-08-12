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

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

func (c *Component) docsHandlerFunc(w http.ResponseWriter, req *http.Request) {
	docs := c.embedOrLiveFS("data/docs")
	requestedDocument := req.PathValue("name")
	if replacement, ok := renamedDocuments[requestedDocument]; ok {
		requestedDocument = replacement
	}
	w.Header().Set("Vary", "Accept")

	// Unless JSON is preferred, answer with the source: this is the most useful
	// answer for a crawler or an LLM.
	if !prefersOverMarkdown(req.Header.Get("Accept"), docsTypeJSON) {
		markdown := c.findDocument(requestedDocument)
		if markdown == nil {
			httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "Document not found."})
			return
		}
		w.Header().Set("Cache-Control", "max-age=300, public")
		writeMarkdownDocument(w, markdown)
		return
	}

	var markdown []byte
	toc := []DocumentTOC{}

	// Find right file and compute ToC
	entries, err := fs.ReadDir(docs, ".")
	if err != nil {
		c.r.Err(err).Msg("unable to list documentation files")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to get documentation files."})
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := internalLinkRegexp.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		f, err := http.FS(docs).Open(entry.Name())
		if err != nil {
			c.r.Err(err).Str("path", entry.Name()).Msg("unable to open documentation file")
			continue
		}

		content, _ := io.ReadAll(f)
		f.Close()
		if matches[3] == requestedDocument {
			// That's the one we need to do final rendering on.
			markdown = content
		}

		// Markdown rendering to build ToC
		tocLogger := &tocLogger{}
		md := goldmark.New(
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
				parser.WithASTTransformers(
					util.Prioritized(tocLogger, 500),
				),
			),
		)
		var buf strings.Builder
		if err = md.Convert(content, &buf); err != nil {
			c.r.Err(err).Str("path", entry.Name()).Msg("unable to render markdown document")
			continue
		}
		toc = append(toc, DocumentTOC{
			Name:    matches[3],
			Section: documentSection(matches[2]),
			Headers: tocLogger.headers,
		})
	}

	if markdown == nil {
		httpserver.WriteJSON(w, http.StatusNotFound, helpers.M{"message": "Document not found."})
		return
	}
	w.Header().Set("Cache-Control", "max-age=300, public")
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Footnote,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithCustomStyle(draculaStyle),
			),
			&admonitionExtension{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&internalLinkTransformer{}, 500),
				util.Prioritized(&imageLinkTransformer{}, 500),
			),
		),
	)
	var buf strings.Builder
	if err = md.Convert(markdown, &buf); err != nil {
		c.r.Err(err).Str("path", requestedDocument).Msg("unable to render markdown document")
		httpserver.WriteJSON(w, http.StatusInternalServerError, helpers.M{"message": "Unable to render document."})
		return
	}

	// Because of the <base> tag, the browser resolves a link to an anchor from
	// the base URL instead of from the current page: name the page in each of
	// them. This is done on the rendered document to also cover the links built
	// by the footnote extension, which does it in its renderer.
	rendered := strings.ReplaceAll(buf.String(), `href="#`,
		fmt.Sprintf(`href="docs/%s#`, requestedDocument))

	httpserver.WritePureJSON(w, http.StatusOK, helpers.M{
		"markdown": rendered,
		"toc":      toc,
	})
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

// internalLinkTransformer rewrites the links to the other documents to URLs
// relative to the <base> tag of the web interface, which is where the browser
// resolves them from.
type internalLinkTransformer struct{}

func (r *internalLinkTransformer) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	replaceLinks := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Link:
			matches := internalLinkRegexp.FindStringSubmatch(string(node.Destination))
			if matches != nil {
				node.Destination = fmt.Appendf(nil, "docs/%s%s", matches[3], matches[4])
			}
		}
		return ast.WalkContinue, nil
	}
	ast.Walk(node, replaceLinks)
}

// imageLinkTransformer rewrites the images of the documentation to URLs
// relative to the <base> tag of the web interface.
type imageLinkTransformer struct{}

func (r *imageLinkTransformer) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	replaceLinks := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Image:
			path := string(node.Destination)
			if !strings.Contains(path, "/") {
				node.Destination = fmt.Appendf(nil, "assets/docs/%s", path)
			}
		}
		return ast.WalkContinue, nil
	}
	ast.Walk(node, replaceLinks)
}

type tocLogger struct {
	headers []Header
}

func (r *tocLogger) Transform(node *ast.Document, reader text.Reader, _ parser.Context) {
	r.headers = []Header{}
	logHeaders := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Heading:
			id, ok := n.AttributeString("id")
			if ok {
				var title []byte
				lastIndex := node.Lines().Len() - 1
				if lastIndex > -1 {
					lastLine := node.Lines().At(lastIndex)
					title = lastLine.Value(reader.Source())
				}
				if title != nil {
					r.headers = append(r.headers, Header{
						ID:    string(id.([]uint8)),
						Level: node.Level,
						Title: string(title),
					})
				}
			}
		}
		return ast.WalkContinue, nil
	}
	ast.Walk(node, logHeaders)
}
