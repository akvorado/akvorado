// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

func TestCacheByRequestPath(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	count := 0
	h.GinRouter.GET("/api/v0/test",
		h.CacheByRequestPath(time.Minute),
		func(c *gin.Context) {
			count++
			c.JSON(http.StatusOK, gin.H{
				"message": "ping",
				"count":   count,
			})
		})

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "not cached",
			URL:         "/api/v0/test",
			JSONOutput:  gin.H{"message": "ping", "count": 1},
		}, {
			Description: "cached",
			URL:         "/api/v0/test",
			JSONOutput:  gin.H{"message": "ping", "count": 1},
		}, {
			Description: "cached twice",
			URL:         "/api/v0/test",
			JSONOutput:  gin.H{"message": "ping", "count": 1},
		},
	})

	gotMetrics := r.GetMetrics("akvorado_common_httpserver_", "requests_", "cache_")
	expectedMetrics := map[string]string{
		`cache_hit_total{method="GET",path="/api/v0/test"}`:       "2",
		`cache_miss_total{method="GET",path="/api/v0/test"}`:      "1",
		`requests_total{code="200",handler="/api/",method="get"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestCacheByRequestBody(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	count := 0
	h.GinRouter.POST("/api/v0/test",
		h.CacheByRequestBody(time.Minute),
		func(c *gin.Context) {
			count++
			data, err := c.GetRawData()
			if err != nil {
				t.Fatalf("GetRawData() error:\n%+v", err)
			}
			c.JSON(http.StatusOK, gin.H{
				"message": "ping",
				"count":   count,
				"body":    string(data),
			})
		})

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "not cached",
			URL:         "/api/v0/test",
			JSONInput:   gin.H{"hop": 1},
			JSONOutput:  gin.H{"message": "ping", "count": 1, "body": `{"hop":1}` + "\n"},
		}, {
			Description: "cached",
			URL:         "/api/v0/test",
			JSONInput:   gin.H{"hop": 1},
			JSONOutput:  gin.H{"message": "ping", "count": 1, "body": `{"hop":1}` + "\n"},
		}, {
			Description: "different body",
			URL:         "/api/v0/test",
			JSONInput:   gin.H{"hop": 2},
			JSONOutput:  gin.H{"message": "ping", "count": 2, "body": `{"hop":2}` + "\n"},
		}, {
			Description: "different body cached",
			URL:         "/api/v0/test",
			JSONInput:   gin.H{"hop": 2},
			JSONOutput:  gin.H{"message": "ping", "count": 2, "body": `{"hop":2}` + "\n"},
		},
	})

	gotMetrics := r.GetMetrics("akvorado_common_httpserver_", "requests_", "cache_")
	expectedMetrics := map[string]string{
		`cache_hit_total{method="POST",path="/api/v0/test"}`:       "2",
		`cache_miss_total{method="POST",path="/api/v0/test"}`:      "2",
		`requests_total{code="200",handler="/api/",method="post"}`: "4",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestRedis(t *testing.T) {
	server := helpers.CheckExternalService(t, "Redis", []string{"redis", "localhost"}, "6379")
	client := redis.NewClient(&redis.Options{
		Addr: server,
		DB:   10,
	})
	defer client.Close()
	if err := client.FlushAll(context.Background()).Err(); err != nil {
		t.Fatalf("FlushAll() error:\n%+v", err)
	}

	r := reporter.NewMock(t)

	// HTTP with Redis
	config := httpserver.DefaultConfiguration()
	config.Listen = "127.0.0.1:0"
	config.Cache.Config = httpserver.RedisCacheConfiguration{
		Protocol: "tcp",
		Server:   server,
		DB:       10,
	}
	h, err := httpserver.New(r, config, httpserver.Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, h)

	count := 0
	h.GinRouter.GET("/api/v0/test",
		h.CacheByRequestPath(time.Minute),
		func(c *gin.Context) {
			count++
			c.JSON(http.StatusOK, gin.H{
				"message": "ping",
				"count":   count,
			})
		})

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "not cached",
			URL:         "/api/v0/test",
			JSONOutput:  gin.H{"message": "ping", "count": 1},
		}, {
			Description: "cached",
			URL:         "/api/v0/test",
			JSONOutput:  gin.H{"message": "ping", "count": 1},
		},
	})

	if err := client.Get(context.Background(), "cache-/api/v0/test").Err(); err != nil {
		t.Fatalf("GET(\"cache-/api/v0/test\") error:\n%+v", err)
	}
}
