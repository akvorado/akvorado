// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver_test

import (
	"fmt"
	"net/http"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestHandler(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	h.AddHandler("/test",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello !")
		}))

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:         "/test",
			ContentType: "text/plain; charset=utf-8",
			FirstLines:  []string{"Hello !"},
		},
	})

	gotMetrics := r.GetMetrics("akvorado_common_httpserver_",
		"inflight_", "requests_total", "response_size")
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
	h := httpserver.NewMock(t, r)

	h.GinRouter.GET("/api/v0/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ping",
		})
	})

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/test",
			ContentType: "application/json; charset=utf-8",
			FirstLines:  []string{`{"message":"ping"}`},
		}, {
			URL:        "/api/v0/test",
			JSONOutput: gin.H{"message": "ping"},
		},
	})
}

func TestGinRouterPanic(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	h.GinRouter.GET("/api/v0/test", func(c *gin.Context) {
		panic("heeeelp")
	})

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/test",
			StatusCode:  500,
			ContentType: "",
			FirstLines:  []string{},
		},
	})
}
