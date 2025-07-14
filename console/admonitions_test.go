// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

func TestAdmonition(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(&admonitionExtension{}),
	)

	tests := []struct {
		name        string
		input       string
		contains    []string
		notContains []string
	}{
		{
			name: "IMPORTANT admonition with content",
			input: `> [!IMPORTANT]
> This is important information.
> It spans multiple lines.`,
			contains: []string{
				`class="admonition admonition-important"`,
				`Important</p>`,
				`<p>This is important information.`,
				`It spans multiple lines.</p>`,
			},
		},
		{
			name: "NOTE admonition",
			input: `> [!NOTE]
> This is a note.`,
			contains: []string{
				`class="admonition admonition-note"`,
				`Note</p>`,
				`<p>This is a note.</p>`,
			},
		},
		{
			name: "TIP admonition",
			input: `> [!TIP]
> This is a tip.`,
			contains: []string{
				`class="admonition admonition-tip"`,
				`Tip</p>`,
				`<p>This is a tip.</p>`,
			},
		},
		{
			name: "WARNING admonition",
			input: `> [!WARNING]
> This is a warning.`,
			contains: []string{
				`class="admonition admonition-warning"`,
				`Warning</p>`,
				`<p>This is a warning.</p>`,
			},
		},
		{
			name: "CAUTION admonition",
			input: `> [!CAUTION]
> This is a caution.`,
			contains: []string{
				`class="admonition admonition-caution"`,
				`Caution</p>`,
				`<p>This is a caution.</p>`,
			},
		},
		{
			name: "CAUTION and TIP adominitions",
			input: `
This is just a text.

> [!CAUTION]
> This is a caution.

> [!TIP]
> This is a tip.
`,
			contains: []string{
				`class="admonition admonition-caution"`,
				`Caution</p>`,
				`<p>This is a caution.</p>`,
				`class="admonition admonition-tip"`,
				`Tip</p>`,
				`<p>This is a tip.</p>`,
			},
		},
		{
			name: "Regular blockquote should not be affected",
			input: `> This is a regular blockquote.
> It should not be styled as an admonition.`,
			contains: []string{
				`<blockquote>`,
				`<p>This is a regular blockquote.`,
			},
		},
		{
			name: "Links inside admonition",
			input: `> [!NOTE]
> Check the [configuration guide](config.md) for more details.`,
			contains: []string{
				`class="admonition admonition-note"`,
				`Note</p>`,
				`<p>Check the <a href="config.md">configuration guide</a> for more details.</p>`,
			},
			notContains: []string{
				`[!NOTE]`,
				`<blockquote>`,
			},
		},
		{
			name: "Emphasis inside admonition",
			input: `> [!WARNING]
> This is *very* important and **must** be done.`,
			contains: []string{
				`class="admonition admonition-warning"`,
				`Warning</p>`,
				`<p>This is <em>very</em> important and <strong>must</strong> be done.</p>`,
			},
			notContains: []string{
				`[!WARNING]`,
				`<blockquote>`,
			},
		},
		{
			name: "Code inside admonition",
			input: `> [!TIP]
> Use the` + " `docker compose up` " + `command to start the services.`,
			contains: []string{
				`class="admonition admonition-tip"`,
				`Tip</p>`,
				`<p>Use the <code>docker compose up</code> command to start the services.</p>`,
			},
			notContains: []string{
				`[!TIP]`,
				`<blockquote>`,
			},
		},
		{
			name: "Multiple markdown features",
			input: `> [!IMPORTANT]
> Read the **[documentation](docs.md)** carefully.
> The` + " `config.yaml` " + `file is *essential*.`,
			contains: []string{
				`class="admonition admonition-important"`,
				`Important</p>`,
				`<p>Read the <strong><a href="docs.md">documentation</a></strong> carefully.`,
				`The <code>config.yaml</code> file is <em>essential</em>.</p>`,
			},
			notContains: []string{
				`[!IMPORTANT]`,
				`<blockquote>`,
			},
		},
		{
			name: "List inside admonition",
			input: `> [!NOTE]
> Follow these steps:
> 1. First step
> 2. Second step with **bold** text`,
			contains: []string{
				`class="admonition admonition-note"`,
				`Note</p>`,
				`<p>Follow these steps:</p>`,
				`<ol>`,
				`<li>First step</li>`,
				`<li>Second step with <strong>bold</strong> text</li>`,
				`</ol>`,
			},
			notContains: []string{
				`[!NOTE]`,
				`<blockquote>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := md.Convert([]byte(tt.input), &buf); err != nil {
				t.Errorf("Convert() error:\n%+v", err)
				return
			}

			html := buf.String()

			for _, expected := range tt.contains {
				if !strings.Contains(html, expected) {
					t.Errorf("Convert() should have %q:\n%s", expected, html)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(html, notExpected) {
					t.Errorf("Convert() should not have %q:\n%s", notExpected, html)
				}
			}
		})
	}
}
