// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	//go:embed data/docs
	embeddedDocs       embed.FS
	internalLinkRegexp = regexp.MustCompile("^(([0-9]+)-([a-z]+).md)(#.*|$)")
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
	Headers []Header `json:"headers"`
}

func (c *Component) docsHandlerFunc(gc *gin.Context) {
	docs := c.embedOrLiveFS(embeddedDocs, "data/docs")
	requestedDocument := gc.Param("name")

	var markdown []byte
	toc := []DocumentTOC{}

	// Find right file and compute ToC
	entries, err := fs.ReadDir(docs, ".")
	if err != nil {
		c.r.Err(err).Msg("unable to list documentation files")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to get documentation files."})
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

		// Markdown rendering to build ToC
		content, _ := ioutil.ReadAll(f)
		f.Close()
		if matches[3] == requestedDocument {
			// That's the one we need to do final rendering on.
			markdown = content
		}
		tocLogger := &tocLogger{}
		md := goldmark.New(
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
				parser.WithASTTransformers(
					util.Prioritized(tocLogger, 500),
				),
			),
		)
		buf := &bytes.Buffer{}
		if err = md.Convert(content, buf); err != nil {
			c.r.Err(err).Str("path", entry.Name()).Msg("unable to render markdown document")
			continue
		}
		toc = append(toc, DocumentTOC{
			Name:    matches[3],
			Headers: tocLogger.headers,
		})
	}

	if markdown == nil {
		gc.JSON(http.StatusNotFound, gin.H{"message": "Document not found."})
		return
	}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Footnote,
			extension.Typographer,
			highlighting.NewHighlighting(
				highlighting.WithCustomStyle(draculaStyle),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&internalLinkTransformer{}, 500),
				util.Prioritized(&imageEmbedder{docs}, 500),
			),
		),
	)
	buf := &bytes.Buffer{}
	if err = md.Convert(markdown, buf); err != nil {
		c.r.Err(err).Str("path", requestedDocument).Msg("unable to render markdown document")
		gc.JSON(http.StatusInternalServerError, gin.H{"message": "Unable to render document."})
		return
	}
	gc.Header("Cache-Control", "max-age=300, public")
	gc.PureJSON(http.StatusOK, gin.H{
		"markdown": buf.String(),
		"toc":      toc,
	})
}

type internalLinkTransformer struct{}

func (r *internalLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	replaceLinks := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Link:
			matches := internalLinkRegexp.FindStringSubmatch(string(node.Destination))
			if matches != nil {
				node.Destination = []byte(fmt.Sprintf("%s%s", matches[3], matches[4]))
			}
		}
		return ast.WalkContinue, nil
	}
	ast.Walk(node, replaceLinks)
}

type imageEmbedder struct {
	root fs.FS
}

func (r *imageEmbedder) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	replaceLinks := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Image:
			path := string(node.Destination)
			if strings.Contains(path, "/") || !strings.HasSuffix(path, ".svg") {
				break
			}
			f, err := r.root.Open(path)
			if err != nil {
				break
			}
			content, err := io.ReadAll(f)
			if err != nil {
				break
			}
			encoded := fmt.Sprintf("data:image/svg+xml;base64,%s", base64.StdEncoding.EncodeToString(content))
			node.Destination = []byte(encoded)
		}
		return ast.WalkContinue, nil
	}
	ast.Walk(node, replaceLinks)
}

type tocLogger struct {
	headers []Header
}

func (r *tocLogger) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
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
