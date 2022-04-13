package http_test

import (
	"fmt"
	"io/ioutil"
	netHTTP "net/http"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestHandler(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)

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

	gotMetrics := r.GetMetrics("akvorado_common_http_", "inflight_", "requests_total", "response_size")
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

func TestGinRouter(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)

	h.GinRouter.GET("/api/v0/test", func(c *gin.Context) {
		c.JSON(netHTTP.StatusOK, gin.H{
			"message": "ping",
		})
	})

	resp, err := netHTTP.Get(fmt.Sprintf("http://%s/api/v0/test", h.Address))
	if err != nil {
		t.Fatalf("GET /api/v0/test:\n%+v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/v0/test: got status code %d, not 200", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	expected := `{"message":"ping"}`
	if diff := helpers.Diff(string(body), expected); diff != "" {
		t.Errorf("GET /api/v0/test (-got, +want):\n%s", diff)
	}
}

func TestGinRouterPanic(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)

	h.GinRouter.GET("/api/v0/test", func(c *gin.Context) {
		panic("heeeelp")
	})

	resp, err := netHTTP.Get(fmt.Sprintf("http://%s/api/v0/test", h.Address))
	if err != nil {
		t.Fatalf("GET /api/v0/test:\n%+v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Errorf("GET /api/v0/test: got status code %d, not 500", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	expected := ""
	if diff := helpers.Diff(string(body), expected); diff != "" {
		t.Errorf("GET /api/v0/test (-got, +want):\n%s", diff)
	}
}
