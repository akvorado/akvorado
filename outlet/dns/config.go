// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package dns

import (
	"net/netip"
	"time"

	"akvorado/common/helpers"
)

// Configuration describes the configuration for the DNS enrichment component.
type Configuration struct {
	// Enabled enables reverse DNS enrichment.
	Enabled bool
	// Resolvers is the explicit list of DNS resolvers to query.
	Resolvers []string
	// Timeout is the timeout for one DNS query attempt.
	Timeout time.Duration `validate:"min=1ms"`
	// WaitForInitialResult waits briefly for the first DNS result on cache miss.
	WaitForInitialResult bool
	// InitialTimeout is the maximum time Lookup may wait on a cache miss.
	InitialTimeout time.Duration `validate:"min=1ms"`
	// Attempts is the number of attempts for each resolver.
	Attempts int `validate:"min=1"`
	// MaxConcurrentQueries limits concurrent DNS queries.
	MaxConcurrentQueries int `validate:"min=1"`
	// Cache configures positive and negative DNS cache.
	Cache CacheConfiguration
	// IncludeSubnets limits reverse DNS queries to matching subnets. Empty means all.
	IncludeSubnets []netip.Prefix
	// ExcludeSubnets excludes matching subnets and takes precedence over IncludeSubnets.
	ExcludeSubnets []netip.Prefix
	// TrimSuffixes lists DNS suffixes to remove from resolved names.
	TrimSuffixes []string
}

// CacheConfiguration describes the reverse DNS cache.
type CacheConfiguration struct {
	// MaxEntries is the maximum number of entries in the cache.
	MaxEntries int `validate:"min=1"`
	// MinTTL is the minimum positive cache TTL.
	MinTTL time.Duration `validate:"min=1s,ltefield=MaxTTL"`
	// MaxTTL is the maximum positive cache TTL.
	MaxTTL time.Duration `validate:"min=1s"`
	// NegativeTTL is the cache TTL for negative answers.
	NegativeTTL time.Duration `validate:"min=1s"`
}

// DefaultConfiguration represents the default configuration for the DNS component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Enabled:              false,
		Resolvers:            []string{"127.0.0.1:53"},
		Timeout:              200 * time.Millisecond,
		WaitForInitialResult: false,
		InitialTimeout:       20 * time.Millisecond,
		Attempts:             1,
		MaxConcurrentQueries: 64,
		Cache: CacheConfiguration{
			MaxEntries:  100000,
			MinTTL:      time.Minute,
			MaxTTL:      24 * time.Hour,
			NegativeTTL: 5 * time.Minute,
		},
		IncludeSubnets: []netip.Prefix{},
		ExcludeSubnets: []netip.Prefix{},
		TrimSuffixes:   []string{},
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultConfiguration()))
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultConfiguration().Cache))
}
