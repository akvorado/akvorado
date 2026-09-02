// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

var (
	admonitionIcons = map[string]string{
		"IMPORTANT": `<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />`,
		"NOTE":      `<path stroke-linecap="round" stroke-linejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />`,
		"TIP":       `<path stroke-linecap="round" stroke-linejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 0 0 1.5-.189m-1.5.189a6.01 6.01 0 0 1-1.5-.189m3.75 7.478a12.06 12.06 0 0 1-4.5 0m3.75 2.383a14.406 14.406 0 0 1-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 1 0-7.517 0c.85.493 1.509 1.333 1.509 2.316V18" />`,
		"WARNING":   `<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />`,
		"CAUTION":   `<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m0-10.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.75c0 5.592 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.57-.598-3.75h-.152c-3.196 0-6.1-1.25-8.25-3.286Zm0 13.036h.008v.008H12v-.008Z" />`,
	}

	// admonitionRegexp matches the marker opening an admonition.
	admonitionRegexp = buildAdmonitionRegexp()

	// admonitionHeaders holds the HTML opening each type of admonition.
	admonitionHeaders = buildAdmonitionHeaders()

	// admonitionsKey is where the blockquotes to turn into admonitions are
	// collected while parsing a document.
	admonitionsKey = parser.NewContextKey()

	kindAdmonition = ast.NewNodeKind("Admonition")
)

// buildAdmonitionRegexp accepts the marker of any known type of admonition.
func buildAdmonitionRegexp() *regexp.Regexp {
	types := slices.Collect(maps.Keys(admonitionIcons))
	return regexp.MustCompile(fmt.Sprintf(`^\[!(%s)\]$`, strings.Join(types, "|")))
}

// buildAdmonitionHeaders renders the header of each type of admonition.
func buildAdmonitionHeaders() map[string]string {
	headers := make(map[string]string, len(admonitionIcons))
	for admonitionType, icon := range admonitionIcons {
		lower := strings.ToLower(admonitionType)
		headers[admonitionType] = fmt.Sprintf(`
<div class="admonition admonition-%s" dir="auto">
  <p class="admonition-title" dir="auto">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
      %s
    </svg>
%s%s
  </p>`,
			lower, icon, admonitionType[:1], lower[1:])
	}
	return headers
}

// admonitionNode represents an admonition (kind of like a blockquote with an icon)
type admonitionNode struct {
	ast.BaseBlock
	AdmonitionType string
}

// newAdmonitionNode creates a new admonition node
func newAdmonitionNode(admonitionType string) *admonitionNode {
	n := &admonitionNode{AdmonitionType: admonitionType}
	n.Init(n)
	return n
}

// Dump implements ast.Node
func (n *admonitionNode) Dump(_ []byte) *ast.NodeDump {
	return ast.NewNodeDump(n, map[string]any{"AdmonitionType": n.AdmonitionType})
}

// Kind implements ast.Node
func (n *admonitionNode) Kind() ast.NodeKind {
	return kindAdmonition
}

// admonition is a blockquote to turn into an admonition.
type admonition struct {
	blockquote *ast.Blockquote
	kind       string
}

// admonitionMarkerTransformer removes the marker opening an admonition from
// the paragraph holding it. It runs before inline parsing, so the marker is
// not part of the rendered document.
type admonitionMarkerTransformer struct{}

// Transform implements parser.ParagraphTransformer
func (t *admonitionMarkerTransformer) Transform(node *ast.Paragraph, reader text.Reader, pc parser.Context) {
	blockquote, ok := node.Parent().(*ast.Blockquote)
	if !ok || node.PreviousSibling() != nil {
		return
	}
	lines := node.Source()
	if len(lines) == 0 {
		return
	}
	matches := admonitionRegexp.FindSubmatch(bytes.TrimSpace(lines[0].Bytes(reader.Source())))
	if matches == nil {
		return
	}
	if len(lines) == 1 {
		// The marker was alone, there is nothing left to render.
		blockquote.RemoveChild(node)
	} else {
		node.SetSource(lines[1:])
	}
	admonitions, _ := pc.Get(admonitionsKey).([]admonition)
	pc.Set(admonitionsKey, append(admonitions, admonition{blockquote, string(matches[1])}))
}

// admonitionTransformer replaces the marked blockquotes by admonitions.
type admonitionTransformer struct{}

// Transform implements parser.ASTTransformer
func (t *admonitionTransformer) Transform(_ *ast.Document, _ text.Reader, pc parser.Context) {
	admonitions, _ := pc.Get(admonitionsKey).([]admonition)
	for _, found := range admonitions {
		node := newAdmonitionNode(found.kind)
		node.SetPos(found.blockquote.Pos())
		node.SetBlankPreviousLines(found.blockquote.HasBlankPreviousLines())
		for child := found.blockquote.FirstChild(); child != nil; {
			next := child.NextSibling()
			found.blockquote.RemoveChild(child)
			node.AppendChild(child)
			child = next
		}
		found.blockquote.Parent().ReplaceChild(found.blockquote, node)
	}
}

// renderAdmonition renders an admonition to HTML.
func renderAdmonition(w io.Writer, _ []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	bw := w.(util.BufWriter)
	if entering {
		_, _ = bw.WriteString(admonitionHeaders[node.(*admonitionNode).AdmonitionType])
	} else {
		_, _ = bw.WriteString(`</div>`)
	}
	return ast.WalkContinue, nil
}

// admonitionParserExtension adds admonitions to a parser.
type admonitionParserExtension struct{}

// ParserOptions implements parser.Extension
func (e *admonitionParserExtension) ParserOptions(_ *parser.Config) []parser.Option {
	return []parser.Option{
		parser.WithParagraphTransformers(
			util.Prioritized[parser.ParagraphTransformer](&admonitionMarkerTransformer{}, 500),
		),
		parser.WithASTTransformers(
			util.Prioritized[parser.ASTTransformer](&admonitionTransformer{}, 500),
		),
	}
}

// admonitionHTMLRendererExtension adds admonitions to an HTML renderer.
type admonitionHTMLRendererExtension struct{}

// RendererOptions implements html.Extension
func (e *admonitionHTMLRendererExtension) RendererOptions(_ *html.Config) []html.Option {
	return []html.Option{
		html.WithNodeRenderer(kindAdmonition, html.NodeRendererFunc(renderAdmonition)),
	}
}

var (
	admonitionParser       parser.Extension = &admonitionParserExtension{}
	admonitionHTMLRenderer html.Extension   = &admonitionHTMLRendererExtension{}
)
