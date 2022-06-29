// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"time"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// CacheDuration defines how long to keep cached entries without access
	CacheDuration time.Duration `validate:"min=1m"`
	// CacheRefresh defines how soon to refresh an existing cached entry
	CacheRefresh time.Duration `validate:"eq=0|min=1m,eq=0|gtefield=CacheDuration"`
	// CacheRefreshInterval defines the interval to check for expiration/refresh
	CacheCheckInterval time.Duration `validate:gtefield=CacheRefresh"`
	// CachePersist defines a file to store cache and survive restarts
	CachePersistFile string
	// DefaultCommunity is the default SNMP community to use
	DefaultCommunity string `validate:"required"`
	// Communities is a mapping from exporter IPs to communities
	Communities map[string]string
	// PollerRetries tell how many time a poller should retry before giving up
	PollerRetries int `validate:"min=0"`
	// PollerTimeout tell how much time a poller should wait for an answer
	PollerTimeout time.Duration
	// PollerCoalesce tells how many requests can be contained inside a single SNMP PDU
	PollerCoalesce int `validate:"min=0"`
	// Workers define the number of workers used to poll SNMP
	Workers int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the SNMP client.
func DefaultConfiguration() Configuration {
	return Configuration{
		CacheDuration:      30 * time.Minute,
		CacheRefresh:       time.Hour,
		CacheCheckInterval: 2 * time.Minute,
		CachePersistFile:   "",
		DefaultCommunity:   "public",
		Communities:        map[string]string{},
		PollerRetries:      1,
		PollerTimeout:      time.Second,
		PollerCoalesce:     10,
		Workers:            1,
	}
}
