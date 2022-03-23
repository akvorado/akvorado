package snmp

import (
	"time"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// CacheDuration defines how long to keep cached entries without access
	CacheDuration time.Duration
	// CacheRefresh defines how soon to refresh an existing cached entry
	CacheRefresh time.Duration
	// CacheRefreshInterval defines the interval to check for expiration/refresh
	CacheCheckInterval time.Duration
	// CachePersist defines a file to store cache and survive restarts
	CachePersistFile string
	// DefaultCommunity is the default SNMP community to use
	DefaultCommunity string
	// Communities is a mapping from sampler IPs to communities
	Communities map[string]string
	// Workers define the number of workers used to poll SNMP
	Workers int
}

// DefaultConfiguration represents the default configuration for the SNMP client.
var DefaultConfiguration = Configuration{
	CacheDuration:      30 * time.Minute,
	CacheRefresh:       time.Hour,
	CacheCheckInterval: 2 * time.Minute,
	CachePersistFile:   "",
	DefaultCommunity:   "public",
	Communities:        map[string]string{},
	Workers:            1,
}
