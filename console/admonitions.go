// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	admonitionRegexp = regexp.MustCompile(`^\[!(IMPORTANT|NOTE|TIP|WARNING|CAUTION)\]$`)
	admonitionIcons  = map[string]string{
		"IMPORTANT": `<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />`,
		"NOTE":      `<path stroke-linecap="round" stroke-linejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />`,
		"TIP":       `<path stroke-linecap="round" stroke-linejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 0 0 1.5-.189m-1.5.189a6.01 6.01 0 0 1-1.5-.189m3.75 7.478a12.06 12.06 0 0 1-4.5 0m3.75 2.383a14.406 14.406 0 0 1-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 1 0-7.517 0c.85.493 1.509 1.333 1.509 2.316V18" />`,
		"WARNING":   `<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />`,
		"CAUTION":   `<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m0-10.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.75c0 5.592 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.57-.598-3.75h-.152c-3.196 0-6.1-1.25-8.25-3.286Zm0 13.036h.008v.008H12v-.008Z" />`,
	}
	kindAdmonition = ast.NewNodeKind("Admonition")
)

// admonitionNode represents an admonition (kind of like a blockquote with an icon)
type admonitionNode struct {
	ast.BaseBlock
	AdmonitionType string
}

// newAdmonitionNode creates a new admonition node
func newAdmonitionNode(admonitionType string) *admonitionNode {
	return &admonitionNode{
		AdmonitionType: admonitionType,
	}
}

// Dump implements ast.Node
func (n *admonitionNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements ast.Node
func (n *admonitionNode) Kind() ast.NodeKind {
	return kindAdmonition
}

// admonitionTransformer converts blockquotes to admonitions.
type admonitionTransformer struct{}

// Transform transforms the AST to turns blockquotes into admonitions.
func (t *admonitionTransformer) Transform(node *ast.Document, reader text.Reader, _ parser.Context) {
	walker := func(n ast.Node) ast.WalkStatus {
		blockquote, ok := n.(*ast.Blockquote)
		if !ok {
			return ast.WalkContinue
		}

		// Check if the first child is a paragraph with admonition syntax
		firstChild := blockquote.FirstChild()
		if firstChild == nil {
			return ast.WalkSkipChildren
		}
		paragraph, ok := firstChild.(*ast.Paragraph)
		if !ok {
			return ast.WalkSkipChildren
		}
		lines := paragraph.Lines()
		if lines.Len() == 0 {
			return ast.WalkSkipChildren
		}
		firstLine := lines.At(0)
		lineContent := bytes.TrimSpace(firstLine.Value(reader.Source()))
		matches := admonitionRegexp.FindSubmatch(lineContent)
		if matches == nil {
			return ast.WalkSkipChildren
		}

		// Recreate the paragraph with the admonition marker removed. We assume it was alone on its line.
		newSegments := text.NewSegments()
		for i := 1; i < lines.Len(); i++ {
			line := lines.At(i)
			newSegments.Append(line)
		}
		paragraph.SetLines(newSegments)
		for child := paragraph.FirstChild(); child != nil; {
			next := child.NextSibling()
			if s, ok := child.(*ast.Text); ok && s.Segment.Stop <= firstLine.Stop {
				paragraph.RemoveChild(paragraph, child)
			}
			child = next
		}

		// Move all children from blockquote to admonition and replace the
		// blockquote with admonition.
		admonitionType := string(matches[1])
		admonitionNode := newAdmonitionNode(admonitionType)
		admonitionNode.AppendChild(admonitionNode, paragraph)
		for child := blockquote.FirstChild(); child != nil; {
			next := child.NextSibling()
			blockquote.RemoveChild(blockquote, child)
			admonitionNode.AppendChild(admonitionNode, child)
			child = next
		}
		parent := blockquote.Parent()
		parent.ReplaceChild(parent, blockquote, admonitionNode)

		return ast.WalkSkipChildren
	}

	var walk func(ast.Node) ast.WalkStatus
	// This is almost like ast.Walk(), except it keeps a reference to the next
	// sibling in case the current element gets removed. We can switch back to
	// ast.Walk() once https://github.com/yuin/goldmark/pull/523 is merged.
	walk = func(n ast.Node) ast.WalkStatus {
		status := walker(n)
		if status == ast.WalkStop {
			return status
		}
		if status != ast.WalkSkipChildren {
			for c := n.FirstChild(); c != nil; {
				next := c.NextSibling()
				if st := walk(c); st == ast.WalkStop {
					return st
				}
				c = next
			}
		}
		return ast.WalkContinue
	}

	walk(node)
}

// admonitionRenderer renders admonitions to HTML
type admonitionRenderer struct{}

// RegisterFuncs implements renderer.NodeRenderer
func (r *admonitionRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindAdmonition, r.renderAdmonition)
}

func (r *admonitionRenderer) renderAdmonition(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*admonitionNode)

	if entering {
		admonitionClass := "admonition admonition-" + strings.ToLower(n.AdmonitionType)
		admonitionIcon := admonitionIcons[n.AdmonitionType]

		_, _ = w.WriteString(`<div class="` + admonitionClass + `" dir="auto">`)
		_, _ = w.WriteString(`<p class="admonition-title" dir="auto">`)
		_, _ = w.WriteString(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">`)
		_, _ = w.WriteString(admonitionIcon)
		_, _ = w.WriteString(`</svg>`)

		title := strings.ToLower(n.AdmonitionType)
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:]
		}
		_, _ = w.WriteString(title)
		_, _ = w.WriteString(`</p>`)
	} else {
		_, _ = w.WriteString(`</div>`)
	}

	return ast.WalkContinue, nil
}

// admonitionExtension extends goldmark with admonitions
type admonitionExtension struct{}

// Extend implements goldmark.Extender
func (e *admonitionExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(&admonitionTransformer{}, 500),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&admonitionRenderer{}, 500),
		),
	)
}
