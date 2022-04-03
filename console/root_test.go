package console

import (
	"fmt"
	"io/ioutil"
	netHTTP "net/http"
	"net/http/httptest"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestProxy(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(
		netHTTP.HandlerFunc(
			func(w netHTTP.ResponseWriter, r *netHTTP.Request) {
				fmt.Fprintf(w, "hello world!")
			}))
	defer server.Close()

	r := reporter.NewMock(t)
	h := http.NewMock(t, r)
	_, err := New(r, Configuration{
		GrafanaURL: server.URL,
	}, Dependencies{
		HTTP: h,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Check the proxy works as expected
	resp, err := netHTTP.Get(fmt.Sprintf("http://%s/grafana/test", h.Address))
	if err != nil {
		t.Fatalf("GET /grafana/test:\n%+v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("GET /grafana/test: cannot read body:\n%+v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /grafana/test: got status code %d, not 200", resp.StatusCode)
	}
	if diff := helpers.Diff(string(body), "hello world!"); diff != "" {
		t.Errorf("GET /grafana/test (-got, +want):\n%s", diff)
	}
}
