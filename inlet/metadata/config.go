// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import (
	"time"

	"akvorado/common/helpers"
	"akvorado/inlet/metadata/provider"
	"akvorado/inlet/metadata/provider/snmp"
)

// Configuration describes the configuration for the metadata client
type Configuration struct {
	// CacheDuration defines how long to keep cached entries without access
	CacheDuration time.Duration `validate:"min=1m"`
	// CacheRefresh defines how soon to refresh an existing cached entry
	CacheRefresh time.Duration `validate:"eq=0|min=1m,eq=0|gtefield=CacheDuration"`
	// CacheRefreshInterval defines the interval to check for expiration/refresh
	CacheCheckInterval time.Duration `validate:"ltefield=CacheRefresh,min=1s"`
	// CachePersist defines a file to store cache and survive restarts
	CachePersistFile string

	// Provider defines the configuration of the provider to sue
	Provider ProviderConfiguration `validate:"dive"`

	// Workers define the number of workers used to poll metadata
	Workers int `validate:"min=1"`
	// MaxBatchRequests define how many requests to pass to a worker at once if possible
	MaxBatchRequests int `validate:"min=0"`
}

// DefaultConfiguration represents the default configuration for the metadata provider.
func DefaultConfiguration() Configuration {
	return Configuration{
		CacheDuration:      30 * time.Minute,
		CacheRefresh:       time.Hour,
		CacheCheckInterval: 2 * time.Minute,
		CachePersistFile:   "",
		Workers:            1,
		MaxBatchRequests:   10,
	}
}

// ProviderConfiguration represents the configuration for a metadata provider.
type ProviderConfiguration struct {
	// Config is the actual configuration for the provider.
	Config provider.Configuration
}

var providers = map[string](func() provider.Configuration){
	"snmp": snmp.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(ProviderConfiguration{}, providers))
}
