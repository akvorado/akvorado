package web

import (
	"fmt"
	"io/ioutil"
	netHTTP "net/http"
	"strings"
	"testing"

	"akvorado/http"
	"akvorado/reporter"
)

func TestServeDocs(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "livefs"
		if !live {
			name = "embeddedfs"
		}
		t.Run(name, func(t *testing.T) {
			r := reporter.NewMock(t)
			h := http.NewMock(t, r)
			_, err := New(r, Configuration{
				ServeLiveFS: live,
			}, Dependencies{HTTP: h})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}

			resp, err := netHTTP.Get(fmt.Sprintf("http://%s/docs/usage", h.Address))
			if err != nil {
				t.Fatalf("GET /docs/usage:\n%+v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("GET /docs/usage: got status code %d, not 200", resp.StatusCode)
			}
			body, _ := ioutil.ReadAll(resp.Body)
			if strings.Contains(string(body), "configuration.md") {
				t.Errorf("GET /docs/usage: contains %q while it should not", "configuration.md")
			}
		})
	}
}
