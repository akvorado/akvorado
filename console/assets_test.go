package console

import (
	"fmt"
	netHTTP "net/http"
	"testing"

	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestServeAssets(t *testing.T) {
	for _, live := range []bool{false, true} {
		cases := []struct {
			Path string
			Code int
		}{
			{"", 200},
			{"something", 200},
			{"assets/akvorado.399701ee.svg", 200},
			{"assets/somethingelse.svg", 404},
		}
		for _, tc := range cases {
			var name string
			switch live {
			case true:
				name = fmt.Sprintf("livefs-%s", tc.Path)
			case false:
				name = fmt.Sprintf("embeddedfs-%s", tc.Path)
			}
			t.Run(name, func(t *testing.T) {
				r := reporter.NewMock(t)
				h := http.NewMock(t, r)
				_, err := New(r, Configuration{
					ServeLiveFS: live,
				}, Dependencies{
					HTTP: h,
				})
				if err != nil {
					t.Fatalf("New() error:\n%+v", err)
				}

				resp, err := netHTTP.Get(fmt.Sprintf("http://%s/%s", h.Address, tc.Path))
				if err != nil {
					t.Fatalf("GET /%s:\n%+v", tc.Path, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != tc.Code {
					t.Errorf("GET /%s: got status code %d, not %d", tc.Path, resp.StatusCode, tc.Code)
				}
			})
		}
	}
}
