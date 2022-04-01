package console

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestTemplate(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "livefs"
		if !live {
			name = "embeddedfs"
		}
		t.Run(name, func(t *testing.T) {
			r := reporter.NewMock(t)
			c, err := New(r, Configuration{
				ServeLiveFS: live,
			}, Dependencies{
				HTTP:   http.NewMock(t, r),
				Daemon: daemon.NewMock(t),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			if err := c.Start(); err != nil {
				t.Fatalf("Start() error:\n%+v", err)
			}
			defer func() {
				if err := c.Stop(); err != nil {
					t.Fatalf("Stop() error:\n%+v", err)
				}
			}()

			w := httptest.NewRecorder()
			c.renderTemplate(w, "dummy.html", templateBaseData{
				RootPath: ".",
			})

			if w.Code != 200 {
				t.Errorf("renderTemplate() code was %d, expected 200", w.Code)
			}
			body := strings.TrimSpace(w.Body.String())
			if !strings.HasPrefix(body, "<!doctype html>") {
				t.Errorf("renderTemplate() body should contain <!doctype html>, got:\n%s",
					body)
			}

			if live && !testing.Short() {
				// Wait for refresh of templates to happen.
				time.Sleep(200 * time.Millisecond)
			}
		})
	}
}
