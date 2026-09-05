// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver_test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"

	"github.com/redis/go-redis/v9"
)

func TestCacheByRequestPath(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	count := 0
	h.APIRouter.GET("/api/v0/test",
		func(w http.ResponseWriter, _ *http.Request) {
			count++
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{
				"message": "ping",
				"count":   count,
			})
		},
		h.CacheByRequestPath(time.Minute))

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "not cached",
			URL:         "/api/v0/test",
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "cached",
			URL:         "/api/v0/test",
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "cached twice",
			URL:         "/api/v0/test",
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
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

// discriminateByRemoteUser makes the caches below key on the `Remote-User'
// header, the way the console does with the login of the authenticated user.
func discriminateByRemoteUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := httpserver.WithCacheKeyDiscriminator(req.Context(),
			req.Header.Get("Remote-User"))
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func TestCacheByRequestPathWithDiscriminator(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	count := 0
	h.APIRouter.GET("/api/v0/test",
		func(w http.ResponseWriter, _ *http.Request) {
			count++
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{
				"message": "ping",
				"count":   count,
			})
		},
		discriminateByRemoteUser, h.CacheByRequestPath(time.Minute))

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "first user",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "first user, cached",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "second user does not get the response of the first one",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"bernard"}},
			JSONOutput:  helpers.M{"message": "ping", "count": 2},
		}, {
			Description: "second user, cached",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"bernard"}},
			JSONOutput:  helpers.M{"message": "ping", "count": 2},
		}, {
			Description: "first user still has their own response",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		},
	})
}

func TestCacheByRequestBodyWithDiscriminator(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	count := 0
	h.APIRouter.POST("/api/v0/test",
		func(w http.ResponseWriter, _ *http.Request) {
			count++
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{
				"message": "ping",
				"count":   count,
			})
		},
		discriminateByRemoteUser, h.CacheByRequestBody(time.Minute))

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "first user",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONInput:   helpers.M{"hop": 1},
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "first user, cached",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONInput:   helpers.M{"hop": 1},
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "second user, same body, does not get the response of the first one",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"bernard"}},
			JSONInput:   helpers.M{"hop": 1},
			JSONOutput:  helpers.M{"message": "ping", "count": 2},
		}, {
			Description: "first user still has their own response",
			URL:         "/api/v0/test",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONInput:   helpers.M{"hop": 1},
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		},
	})
}

func TestCacheByRequestBody(t *testing.T) {
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)

	count := 0
	h.APIRouter.POST("/api/v0/test",
		func(w http.ResponseWriter, req *http.Request) {
			count++
			data, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error:\n%+v", err)
			}
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{
				"message": "ping",
				"count":   count,
				"body":    string(data),
			})
		},
		h.CacheByRequestBody(time.Minute))

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "not cached",
			URL:         "/api/v0/test",
			JSONInput:   helpers.M{"hop": 1},
			JSONOutput:  helpers.M{"message": "ping", "count": 1, "body": `{"hop":1}` + "\n"},
		}, {
			Description: "cached",
			URL:         "/api/v0/test",
			JSONInput:   helpers.M{"hop": 1},
			JSONOutput:  helpers.M{"message": "ping", "count": 1, "body": `{"hop":1}` + "\n"},
		}, {
			Description: "different body",
			URL:         "/api/v0/test",
			JSONInput:   helpers.M{"hop": 2},
			JSONOutput:  helpers.M{"message": "ping", "count": 2, "body": `{"hop":2}` + "\n"},
		}, {
			Description: "different body cached",
			URL:         "/api/v0/test",
			JSONInput:   helpers.M{"hop": 2},
			JSONOutput:  helpers.M{"message": "ping", "count": 2, "body": `{"hop":2}` + "\n"},
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
	server := helpers.CheckExternalService(t, "Redis",
		[]string{"redis:6379", "127.0.0.1:6379"})
	client := redis.NewClient(&redis.Options{
		Addr: server,
		DB:   10,
	})
	defer client.Close()
	if err := client.FlushAll(t.Context()).Err(); err != nil {
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
	h, err := httpserver.New(r, "cache-test", config, httpserver.Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, h)

	count := 0
	h.APIRouter.GET("/api/v0/test",
		func(w http.ResponseWriter, _ *http.Request) {
			count++
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{
				"message": "ping",
				"count":   count,
			})
		},
		h.CacheByRequestPath(time.Minute))

	// Check the HTTP server is running and answering metrics
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "not cached",
			URL:         "/api/v0/test",
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		}, {
			Description: "cached",
			URL:         "/api/v0/test",
			JSONOutput:  helpers.M{"message": "ping", "count": 1},
		},
	})

	if err := client.Get(t.Context(), "cache-path-/api/v0/test").Err(); err != nil {
		t.Fatalf("GET(\"cache-path-/api/v0/test\") error:\n%+v", err)
	}
}
