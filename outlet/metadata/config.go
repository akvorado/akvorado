// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import (
	"time"

	"akvorado/common/helpers"
	"akvorado/outlet/metadata/provider"
	"akvorado/outlet/metadata/provider/gnmi"
	"akvorado/outlet/metadata/provider/snmp"
	"akvorado/outlet/metadata/provider/static"
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
	CachePersistFile string `validate:"isdefault|filepath"`

	// Provider defines the configuration of the providers to use
	Providers []ProviderConfiguration

	// QueryTimeout defines how long to wait for a provider to answer.
	QueryTimeout time.Duration `validate:"min=100ms,max=1m"`
	// InitialDelay defines how long to wait at start (when receiving the first
	// packets) before applying the query timeout
	InitialDelay time.Duration `validate:"min=1s,max=1h"`
}

// DefaultConfiguration represents the default configuration for the metadata provider.
func DefaultConfiguration() Configuration {
	return Configuration{
		CacheDuration:      30 * time.Minute,
		CacheRefresh:       time.Hour,
		CacheCheckInterval: 2 * time.Minute,
		QueryTimeout:       5 * time.Second,
		InitialDelay:       time.Minute,
	}
}

// ProviderConfiguration represents the configuration for a metadata provider.
type ProviderConfiguration struct {
	// Config is the actual configuration for the provider.
	Config provider.Configuration
}

// MarshalYAML undoes ConfigurationUnmarshallerHook().
func (pc ProviderConfiguration) MarshalYAML() (any, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(pc, providers)
}

var providers = map[string](func() provider.Configuration){
	"snmp":   snmp.DefaultConfiguration,
	"gnmi":   gnmi.DefaultConfiguration,
	"static": static.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.RenameKeyUnmarshallerHook(Configuration{}, "Provider", "Providers"))
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(ProviderConfiguration{}, providers))
	helpers.RegisterMapstructureDeprecatedFields[Configuration]("Workers", "MaxBatchRequests")
}
