// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"akvorado/common/helpers"
)

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
			{"intro", `../assets/docs/design.svg`},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s-%s", name, tc.Path), func(t *testing.T) {
				conf := DefaultConfiguration()
				conf.ServeLiveFS = live
				_, h, _, _ := NewMock(t, conf)

				resp, err := http.Get(fmt.Sprintf("http://%s/api/v0/console/docs/%s",
					h.LocalAddr(), tc.Path))
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
