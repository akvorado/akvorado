// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package http

import (
	"context"
	"fmt"
	"time"

	"akvorado/common/helpers"

	"github.com/chenyahui/gin-cache/persist"
	"github.com/go-redis/redis/v8"
)

// Configuration describes the configuration for the HTTP server.
type Configuration struct {
	// Listen defines the listening string to listen to.
	Listen string `validate:"required,listen"`
	// Profiler enables Go profiler as /debug
	Profiler bool
	// Cache configuration
	Cache CacheConfiguration
}

// CacheConfiguration describes the configuration of the internal HTTP cache.
// Currently, it delegates everything to the implemented backends.
type CacheConfiguration struct {
	// Config is the backend-specific configuration for the cache
	Config CacheBackendConfiguration
}

// CacheBackendConfiguration represents the configuration of a cache backend.
type CacheBackendConfiguration interface {
	New() (persist.CacheStore, error)
}

// MemoryCacheConfiguration is the configuration for an in-memory cache. There
// is no configuration.
type MemoryCacheConfiguration struct{}

// New creates a new memory cache store from a memory cache configuration.
func (MemoryCacheConfiguration) New() (persist.CacheStore, error) {
	return persist.NewMemoryStore(5 * time.Minute), nil
}

// DefaultMemoryCacheConfiguration returns the default configuration for an
// in-memory cache.
func DefaultMemoryCacheConfiguration() CacheBackendConfiguration {
	return MemoryCacheConfiguration{}
}

// RedisCacheConfiguration is the configuration for a Redis cache.
type RedisCacheConfiguration struct {
	// Protocol to connect with
	Protocol string `validate:"oneof=tcp unix"`
	// Server to connect to (with port)
	Server string `validate:"required,listen"`
	// Optional username
	Username string
	// Optional password
	Password string
	// Database to connect to
	DB int
}

// New creates a new Redis cache store from a Redis cache configuration.
func (c RedisCacheConfiguration) New() (persist.CacheStore, error) {
	client := redis.NewClient(&redis.Options{
		Network:  c.Protocol,
		Addr:     c.Server,
		Username: c.Username,
		Password: c.Password,
		DB:       c.DB,
	})
	// TODO: defer client.Close()
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("cannot ping Redis server: %w", err)
	}
	return persist.NewRedisStore(client), nil
}

// DefaultRedisCacheConfiguration returns the default configuration for a
// Redis-backed cache.
func DefaultRedisCacheConfiguration() CacheBackendConfiguration {
	return RedisCacheConfiguration{
		Protocol: "tcp",
		Server:   "127.0.0.1:6379",
	}
}

// DefaultConfiguration is the default configuration of the HTTP server.
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen: "0.0.0.0:8080",
		Cache: CacheConfiguration{
			Config: DefaultMemoryCacheConfiguration(),
		},
	}
}

// MarshalYAML undoes ConfigurationUnmarshallerHook().
func (cc CacheConfiguration) MarshalYAML() (interface{}, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(cc, cacheConfigurationMap)
}

// MarshalJSON undoes ConfigurationUnmarshallerHook().
func (cc CacheConfiguration) MarshalJSON() ([]byte, error) {
	return helpers.ParametrizedConfigurationMarshalJSON(cc, cacheConfigurationMap)
}

var cacheConfigurationMap = map[string](func() CacheBackendConfiguration){
	"memory": DefaultMemoryCacheConfiguration,
	"redis":  DefaultRedisCacheConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(CacheConfiguration{}, cacheConfigurationMap))
}
