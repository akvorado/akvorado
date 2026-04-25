// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"time"

	"akvorado/common/httpserver/cachestore"
)

// cachedResponse stores all the data needed to replay a cached
// response.
type cachedResponse struct {
	Status  int
	Headers http.Header
	Body    []byte
}

// CacheByRequestPath is a middleware that caches the response keyed
// on the request path.
func (c *Component) CacheByRequestPath(expire time.Duration) Middleware {
	return c.cacheMiddleware(expire, func(req *http.Request) (string, bool) {
		return fmt.Sprintf("cache-path-%s", req.URL.Path), true
	})
}

// CacheByRequestBody is a middleware that caches the response keyed
// on the request method, path and body.
func (c *Component) CacheByRequestBody(expire time.Duration) Middleware {
	return c.cacheMiddleware(expire, func(req *http.Request) (string, bool) {
		body, err := io.ReadAll(req.Body)
		// Restore body for downstream handlers (best-effort, even on error).
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		if err != nil {
			c.r.Error().Err(err).Msg("cannot read body for cache key")
			return "", false
		}
		h := sha256.New()
		fmt.Fprintf(h, "%s\x00%s\x00", req.Method, req.URL.Path)
		h.Write(body)
		return fmt.Sprintf("cache-request-%s", string(h.Sum(nil))), true
	})
}

func (c *Component) cacheMiddleware(expire time.Duration, keyFn func(*http.Request) (string, bool)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key, ok := keyFn(req)
			if !ok {
				next.ServeHTTP(w, req)
				return
			}

			var cached cachedResponse
			if err := c.cacheStore.Get(key, &cached); err == nil {
				c.metrics.cacheHit.WithLabelValues(req.URL.Path, req.Method).Inc()
				dst := w.Header()
				maps.Copy(dst, cached.Headers)
				w.WriteHeader(cached.Status)
				w.Write(cached.Body)
				return
			} else if !errors.Is(err, cachestore.ErrMiss) {
				c.r.Error().Err(err).Msg("cache backend error")
			}

			c.metrics.cacheMiss.WithLabelValues(req.URL.Path, req.Method).Inc()

			// Let the next middleware handle the request and record its result.
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, req)

			result := rec.Result()
			body := rec.Body.Bytes()
			dst := w.Header()
			maps.Copy(dst, result.Header)
			w.WriteHeader(rec.Code)
			w.Write(body)

			// Only cache successful responses.
			if rec.Code >= 200 && rec.Code < 300 {
				if err := c.cacheStore.Set(key, cachedResponse{
					Status:  rec.Code,
					Headers: result.Header,
					Body:    body,
				}, expire); err != nil {
					c.r.Error().Err(err).Msg("cannot store cached response")
				}
			}
		})
	}
}
