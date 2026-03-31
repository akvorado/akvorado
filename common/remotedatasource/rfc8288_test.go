// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasource

import (
	"testing"

	"akvorado/common/helpers"
)

func TestParseLinkHeader(t *testing.T) {
	cases := []struct {
		description string
		header      string
		rel         string
		expected    string
	}{
		{
			description: "simple rel next",
			header:      `<http://example.com/page2>; rel="next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "unquoted rel",
			header:      `<http://example.com/page2>; rel=next`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "rel with multiple types",
			header:      `<http://example.com/page2>; rel="next prefetch"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "second rel type",
			header:      `<http://example.com/page2>; rel="prefetch next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "multiple params before rel",
			header:      `<http://example.com/page2>; title="next chapter"; rel="next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "multiple params after rel",
			header:      `<http://example.com/page2>; rel="next"; title="next chapter"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "multiple links, second has rel next",
			header:      `<http://example.com/page0>; rel="prev", <http://example.com/page2>; rel="next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "no matching rel",
			header:      `<http://example.com/page0>; rel="prev"`,
			rel:         "next",
			expected:    "",
		}, {
			description: "spaces around equals",
			header:      `<http://example.com/page2>; rel = "next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "case-insensitive rel",
			header:      `<http://example.com/page2>; rel="Next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "comma in quoted param",
			header:      `<http://example.com/page2>; title="a, b"; rel="next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "semicolon in quoted param",
			header:      `<http://example.com/page2>; title="a; b"; rel="next"`,
			rel:         "next",
			expected:    "http://example.com/page2",
		}, {
			description: "relative URL",
			header:      `</page2>; rel="next"`,
			rel:         "next",
			expected:    "/page2",
		}, {
			description: "empty header",
			header:      "",
			rel:         "next",
			expected:    "",
		}, {
			description: "RFC 8288 example with encoded title",
			header:      `</TheBook/chapter2>; rel="previous"; title*=UTF-8'de'letztes%20Kapitel, </TheBook/chapter4>; rel="next"; title*=UTF-8'de'n%c3%a4chstes%20Kapitel`,
			rel:         "next",
			expected:    "/TheBook/chapter4",
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got := parseLinkHeader(tc.header, tc.rel)
			if diff := helpers.Diff(got, tc.expected); diff != "" {
				t.Errorf("parseLinkHeader(%q, %q) (-got, +want):\n%s", tc.header, tc.rel, diff)
			}
		})
	}
}
