// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
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
			{"intro", `data:image/svg`},
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
