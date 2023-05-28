// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver

import (
	"bytes"
	"crypto"
	"io/ioutil"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/gin-gonic/gin"

	"akvorado/common/reporter"
)

// CacheByRequestPath is a middleware to cache the request using path as key
func (c *Component) CacheByRequestPath(expire time.Duration) gin.HandlerFunc {
	opts := c.commonCacheOptions()
	opts = append(opts, cache.WithCacheStrategyByRequest(func(gc *gin.Context) (bool, cache.Strategy) {
		return true, cache.Strategy{
			CacheKey: gc.Request.URL.Path,
		}
	}))
	return cache.Cache(c.cacheStore, expire, opts...)
}

// CacheByRequestBody is a middleware to cache the request using body as key
func (c *Component) CacheByRequestBody(expire time.Duration) gin.HandlerFunc {
	opts := c.commonCacheOptions()
	opts = append(opts, cache.WithCacheStrategyByRequest(func(gc *gin.Context) (bool, cache.Strategy) {
		requestBody, err := gc.GetRawData()
		gc.Request.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))
		if err != nil {
			return false, cache.Strategy{}
		}
		h := crypto.SHA256.New()
		bodyHash := string(h.Sum(requestBody))
		return true, cache.Strategy{
			CacheKey: bodyHash,
		}
	}))
	return cache.Cache(c.cacheStore, expire, opts...)
}

func (c *Component) commonCacheOptions() []cache.Option {
	return []cache.Option{
		cache.WithLogger(cacheLogger{c.r}),
		cache.WithOnHitCache(func(gc *gin.Context) {
			c.metrics.cacheHit.WithLabelValues(gc.Request.URL.Path, gc.Request.Method).Inc()
		}),
		cache.WithOnMissCache(func(gc *gin.Context) {
			c.metrics.cacheMiss.WithLabelValues(gc.Request.URL.Path, gc.Request.Method).Inc()
		}),
		cache.WithPrefixKey("cache-"),
	}
}

type cacheLogger struct {
	r *reporter.Reporter
}

func (cl cacheLogger) Errorf(msg string, args ...interface{}) {
	cl.r.Error().Msgf(msg, args...)
}
