package http_test

import (
	"fmt"
	netHTTP "net/http"
	"runtime"
	"testing"

	"flowexporter/http"
	"flowexporter/reporter"
)

func TestHandler(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)
	defer func() {
		h.Stop()
		runtime.Gosched()
		resp, err := netHTTP.Get(fmt.Sprintf("http://%s/", h.Address))
		if err == nil {
			t.Errorf("Still able to connect to expvar server after stop")
			resp.Body.Close()
		}
	}()

	h.AddHandler("/test",
		netHTTP.HandlerFunc(func(w netHTTP.ResponseWriter, r *netHTTP.Request) {
			fmt.Fprintf(w, "Hello !")
		}))

	// Check the HTTP server is running and answering metrics
	resp, err := netHTTP.Get(fmt.Sprintf("http://%s/test", h.Address))
	if err != nil {
		t.Fatalf("GET /test:\n%+v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /test: got status code %d, not 200", resp.StatusCode)
	}
}
