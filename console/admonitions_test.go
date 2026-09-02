// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

func TestAdmonition(t *testing.T) {
	p := parser.New(parser.WithExtensions(admonitionParser))
	r := html.New(html.WithExtensions(admonitionHTMLRenderer))

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
				"\nImportant\n  </p>",
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
				"\nNote\n  </p>",
				`<p>This is a note.</p>`,
			},
		},
		{
			name: "TIP admonition",
			input: `> [!TIP]
> This is a tip.`,
			contains: []string{
				`class="admonition admonition-tip"`,
				"\nTip\n  </p>",
				`<p>This is a tip.</p>`,
			},
		},
		{
			name: "WARNING admonition",
			input: `> [!WARNING]
> This is a warning.`,
			contains: []string{
				`class="admonition admonition-warning"`,
				"\nWarning\n  </p>",
				`<p>This is a warning.</p>`,
			},
		},
		{
			name: "CAUTION admonition",
			input: `> [!CAUTION]
> This is a caution.`,
			contains: []string{
				`class="admonition admonition-caution"`,
				"\nCaution\n  </p>",
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
				"\nCaution\n  </p>",
				`<p>This is a caution.</p>`,
				`class="admonition admonition-tip"`,
				"\nTip\n  </p>",
				`<p>This is a tip.</p>`,
			},
		},
		{
			name:  "Admonition without content",
			input: `> [!NOTE]`,
			contains: []string{
				`class="admonition admonition-note"`,
				`</p></div>`,
			},
			notContains: []string{
				`<p></p>`,
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
				"\nNote\n  </p>",
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
				"\nWarning\n  </p>",
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
				"\nTip\n  </p>",
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
				"\nImportant\n  </p>",
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
				"\nNote\n  </p>",
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
			source := []byte(tt.input)
			if err := r.Render(&buf, source, p.Parse(source)); err != nil {
				t.Errorf("Render() error:\n%+v", err)
				return
			}

			rendered := buf.String()

			for _, expected := range tt.contains {
				if !strings.Contains(rendered, expected) {
					t.Errorf("Render() should have %q:\n%s", expected, rendered)
				}
			}

			for _, notExpected := range tt.notContains {
				if strings.Contains(rendered, notExpected) {
					t.Errorf("Render() should not have %q:\n%s", notExpected, rendered)
				}
			}
		})
	}
}
