package console

import (
	"fmt"
	netHTTP "net/http"
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestServeAssets(t *testing.T) {
	for _, live := range []bool{false, true} {
		for _, f := range []string{"images/akvorado.svg", "javascript/bootstrap.bundle.min.js"} {
			var name string
			switch live {
			case true:
				name = fmt.Sprintf("livefs-%s", f)
			case false:
				name = fmt.Sprintf("embeddedfs-%s", f)
			}
			t.Run(name, func(t *testing.T) {
				r := reporter.NewMock(t)
				h := http.NewMock(t, r)
				_, err := New(r, Configuration{
					ServeLiveFS: live,
				}, Dependencies{
					HTTP:   h,
					Daemon: daemon.NewMock(t),
				})
				if err != nil {
					t.Fatalf("New() error:\n%+v", err)
				}

				resp, err := netHTTP.Get(fmt.Sprintf("http://%s/assets/%s", h.Address, f))
				if err != nil {
					t.Fatalf("GET /assets/%s:\n%+v", f, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != 200 {
					t.Errorf("GET /assets/%s: got status code %d, not 200", f, resp.StatusCode)
				}
			})
		}
	}
}
