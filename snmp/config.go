package snmp

import (
	"time"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// CacheDuration defines how long to keep cached entries
	CacheDuration time.Duration
	// CacheRefresh defines how soon to refresh an existing cached entry
	CacheRefresh time.Duration
	// CacheRefreshInterval defines the interval to use for refresh thread
	CacheRefreshInterval time.Duration
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
	CacheDuration:        time.Hour,
	CacheRefresh:         5 * time.Minute,
	CacheRefreshInterval: 2 * time.Minute,
	CachePersistFile:     "",
	DefaultCommunity:     "public",
	Communities:          map[string]string{},
	Workers:              1,
}
