// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"akvorado/common/helpers"
)

// fetchDoc requests a document with the provided Accept header. An empty header
// is not sent at all.
func fetchDoc(t *testing.T, addr net.Addr, path, accept string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest("GET",
		fmt.Sprintf("http://%s/api/v0/console/docs/%s", addr, path), nil)
	if err != nil {
		t.Fatalf("NewRequest() error:\n%+v", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return http.DefaultClient.Do(req)
}

func TestServeDocs(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "livefs"
		if !live {
			name = "embeddedfs"
		}
		cases := []struct {
			Path   string
			Expect string
		}{
			{"usage", `<a href=\"configuration\">configuration section</a>`},
			{"design", `../assets/docs/design.svg`},
			// Documents which do not exist anymore are served by their
			// replacement.
			{"operations", `<h1 id=\"configure-your-exporters\">`},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s-%s", name, tc.Path), func(t *testing.T) {
				conf := DefaultConfiguration()
				conf.ServeLiveFS = live
				_, h, _, _ := NewMock(t, conf)

				resp, err := fetchDoc(t, h.LocalAddr(), tc.Path, "application/json")
				if err != nil {
					t.Fatalf("GET /api/v0/console/docs/%s:\n%+v", tc.Path, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					t.Errorf("GET /api/v0/console/docs/%s: got status code %d, not 200",
						tc.Path, resp.StatusCode)
				}
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), tc.Expect) {
					t.Logf("Body:\n%s", string(body))
					t.Errorf("GET /api/v0/console/docs/%s: does not contain %q",
						tc.Path, tc.Expect)
				}
			})
		}
	}
}

func TestDocsSections(t *testing.T) {
	conf := DefaultConfiguration()
	_, h, _, _ := NewMock(t, conf)

	resp, err := fetchDoc(t, h.LocalAddr(), "intro", "application/json")
	if err != nil {
		t.Fatalf("GET /api/v0/console/docs/intro:\n%+v", err)
	}
	defer resp.Body.Close()
	var payload struct {
		TOC []struct {
			Name    string `json:"name"`
			Section string `json:"section"`
		} `json:"toc"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("GET /api/v0/console/docs/intro Decode() error:\n%+v", err)
	}

	// Only check one document per section: adding a new document should not
	// break this test.
	interesting := map[string]bool{
		"intro": true, "explore": true, "install": true,
		"configuration": true, "internals": true, "changelog": true,
	}
	got := map[string]string{}
	for _, document := range payload.TOC {
		if interesting[document.Name] {
			got[document.Name] = document.Section
		}
	}
	expected := map[string]string{
		"intro":         "",
		"explore":       "Tutorials",
		"install":       "How-to guides",
		"configuration": "Reference",
		"internals":     "Explanation",
		"changelog":     "",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("GET /api/v0/console/docs/intro sections (-got, +want):\n%s", diff)
	}
}

func TestPrefersOverMarkdown(t *testing.T) {
	cases := []struct {
		Accept    string
		MediaType string
		Expected  bool
	}{
		// A wildcard does not name anything: Markdown wins.
		{"", "application/json", false},
		{"*/*", "application/json", false},
		{"text/*", "text/html", false},
		{"application/xml", "application/json", false},
		{"garbage", "application/json", false},
		// The media type is named.
		{"application/json", "application/json", true},
		{"application/json; charset=utf-8", "application/json", true},
		{"text/markdown, application/json", "application/json", true},
		{"text/markdown", "application/json", false},
		// What a browser sends.
		{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "text/html", true},
		{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "application/json", false},
		// Quality values decide between the two.
		{"text/markdown;q=0.9,text/html;q=1", "text/html", true},
		{"text/markdown;q=0.9, text/html;q=0.1", "text/html", false},
		{"application/json;q=0.1, text/markdown", "application/json", false},
		{"text/html;q=0", "text/html", false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s-%s", tc.Accept, tc.MediaType), func(t *testing.T) {
			got := prefersOverMarkdown(tc.Accept, tc.MediaType)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("prefersOverMarkdown(%q, %q) (-got, +want):\n%s",
					tc.Accept, tc.MediaType, diff)
			}
		})
	}
}

func TestServeAPIDocsMediaTypes(t *testing.T) {
	cases := []struct {
		Name        string
		Accept      string
		Path        string
		ContentType string
		Expect      string
	}{
		{"json", "application/json", "intro", "application/json; charset=utf-8", `"toc":`},
		{"markdown", "text/markdown", "intro", "text/markdown; charset=utf-8", "# Introduction"},
		// Without a preference, the markdown is served
		{"default", "", "intro", "text/markdown; charset=utf-8", "# Introduction"},
		// A browser accepting anything also gets the source on this endpoint.
		{"browser", "text/html,application/xhtml+xml,*/*;q=0.8", "intro",
			"text/markdown; charset=utf-8", "# Introduction"},
		// Links to the other documents point to the matching URLs.
		{"markdown-link", "text/markdown", "collect", "", "](configuration#snmp-provider)"},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			conf := DefaultConfiguration()
			_, h, _, _ := NewMock(t, conf)

			resp, err := fetchDoc(t, h.LocalAddr(), tc.Path, tc.Accept)
			if err != nil {
				t.Fatalf("GET /api/v0/console/docs/%s:\n%+v", tc.Path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("GET /api/v0/console/docs/%s: got status code %d, not 200",
					tc.Path, resp.StatusCode)
			}
			if tc.ContentType != "" {
				if diff := helpers.Diff(resp.Header.Get("Content-Type"), tc.ContentType); diff != "" {
					t.Errorf("GET /api/v0/console/docs/%s Content-Type (-got, +want):\n%s",
						tc.Path, diff)
				}
			}
			if diff := helpers.Diff(resp.Header.Get("Vary"), "Accept"); diff != "" {
				t.Errorf("GET /api/v0/console/docs/%s Vary (-got, +want):\n%s", tc.Path, diff)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), tc.Expect) {
				t.Logf("Body:\n%s", string(body))
				t.Errorf("GET /api/v0/console/docs/%s: does not contain %q", tc.Path, tc.Expect)
			}
		})
	}
}

// The documentation pages of the web interface answer with the source, unless
// the client prefers HTML, which is what a browser asks for.
func TestServeSPADocs(t *testing.T) {
	cases := []struct {
		Name        string
		Path        string
		Accept      string
		ContentType string
		Vary        string
		Expect      string
	}{
		{"markdown", "/docs/intro", "text/markdown",
			"text/markdown; charset=utf-8", "Accept", "# Introduction"},
		// Quality values are respected: HTML is named, but ranked lower.
		{"markdown-preferred", "/docs/intro", "text/markdown;q=0.9, text/html;q=0.1",
			"text/markdown; charset=utf-8", "Accept", "# Introduction"},
		{"html-preferred", "/docs/intro", "text/markdown;q=0.9, text/html;q=0.99",
			"text/html; charset=utf-8", "Accept", "<!doctype html>"},
		// A browser names text/html, so it gets the application.
		{"browser", "/docs/intro", "text/html,application/xhtml+xml,*/*;q=0.8",
			"text/html; charset=utf-8", "Accept", "<!doctype html>"},
		{"html", "/docs/intro", "text/html", "text/html; charset=utf-8", "Accept", "<!doctype html>"},
		// A client accepting anything is not a browser: it gets the source.
		{"anything", "/docs/intro", "*/*", "text/markdown; charset=utf-8", "Accept", "# Introduction"},
		{"no-accept", "/docs/intro", "", "text/markdown; charset=utf-8", "Accept", "# Introduction"},
		// Unknown document: the application displays the error itself.
		{"unknown", "/docs/nonexistent", "text/markdown",
			"text/html; charset=utf-8", "Accept", "<!doctype html>"},
		// Not a documentation page: the answer does not depend on Accept.
		{"other-page", "/visualize", "text/markdown", "text/html; charset=utf-8", "", "<!doctype html>"},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			conf := DefaultConfiguration()
			_, h, _, _ := NewMock(t, conf)

			req, err := http.NewRequest("GET",
				fmt.Sprintf("http://%s%s", h.LocalAddr(), tc.Path), nil)
			if err != nil {
				t.Fatalf("NewRequest() error:\n%+v", err)
			}
			if tc.Accept != "" {
				req.Header.Set("Accept", tc.Accept)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET %s:\n%+v", tc.Path, err)
			}
			defer resp.Body.Close()
			if diff := helpers.Diff(resp.Header.Get("Content-Type"), tc.ContentType); diff != "" {
				t.Errorf("GET %s Content-Type (-got, +want):\n%s", tc.Path, diff)
			}
			if diff := helpers.Diff(resp.Header.Get("Vary"), tc.Vary); diff != "" {
				t.Errorf("GET %s Vary (-got, +want):\n%s", tc.Path, diff)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(strings.ToLower(string(body)), strings.ToLower(tc.Expect)) {
				t.Logf("Body:\n%s", string(body))
				t.Errorf("GET %s: does not contain %q", tc.Path, tc.Expect)
			}
		})
	}
}

func TestServeImages(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "livefs"
		if !live {
			name = "embeddedfs"
		}

		t.Run(name, func(t *testing.T) {
			conf := DefaultConfiguration()
			conf.ServeLiveFS = live
			_, h, _, _ := NewMock(t, conf)

			resp, err := http.Get(fmt.Sprintf("http://%s/assets/docs/design.svg",
				h.LocalAddr()))
			if err != nil {
				t.Fatalf("GET /assets/docs/design.svg:\n%+v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("GET /assets/docs/design.svg: got status code %d, not 200",
					resp.StatusCode)
			}
			expected := `<?xml version="1.0" encoding="UTF-8"?>`
			got := make([]byte, len(expected))
			if _, err := io.ReadFull(resp.Body, got); err != nil {
				t.Fatalf("GET /assets/docs/design.svg ReadFull() error:\n%+v", err)
			}
			if diff := helpers.Diff(string(got), expected); diff != "" {
				t.Errorf("GET /assets/docs/design.svg:\n%s",
					diff)
			}
		})
	}
}
