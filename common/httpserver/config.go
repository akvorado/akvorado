// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package httpserver

import (
	"akvorado/common/helpers"
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

// MemoryCacheConfiguration is the configuration for an in-memory cache. There
// is no configuration.
type MemoryCacheConfiguration struct{}

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
		Listen:   ":8080",
		Profiler: true,
		Cache: CacheConfiguration{
			Config: DefaultMemoryCacheConfiguration(),
		},
	}
}

// MarshalYAML undoes ConfigurationUnmarshallerHook().
func (cc CacheConfiguration) MarshalYAML() (any, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(cc, cacheConfigurationMap)
}

var cacheConfigurationMap = map[string](func() CacheBackendConfiguration){
	"memory": DefaultMemoryCacheConfiguration,
	"redis":  DefaultRedisCacheConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(CacheConfiguration{}, cacheConfigurationMap))
}
