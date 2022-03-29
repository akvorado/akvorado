package http_test

import (
	"fmt"
	netHTTP "net/http"
	"runtime"
	"testing"

	"akvorado/helpers"
	"akvorado/http"
	"akvorado/reporter"
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

	gotMetrics := r.GetMetrics("akvorado_http_", "inflight_", "requests_total", "response_size")
	expectedMetrics := map[string]string{
		`inflight_requests`: "0",
		`requests_total{code="200",handler="/test",method="get"}`:            "1",
		`response_size_bytes_bucket{handler="/test",method="get",le="+Inf"}`: "1",
		`response_size_bytes_bucket{handler="/test",method="get",le="1000"}`: "1",
		`response_size_bytes_bucket{handler="/test",method="get",le="1500"}`: "1",
		`response_size_bytes_bucket{handler="/test",method="get",le="200"}`:  "1",
		`response_size_bytes_bucket{handler="/test",method="get",le="500"}`:  "1",
		`response_size_bytes_bucket{handler="/test",method="get",le="5000"}`: "1",
		`response_size_bytes_count{handler="/test",method="get"}`:            "1",
		`response_size_bytes_sum{handler="/test",method="get"}`:              "7",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
