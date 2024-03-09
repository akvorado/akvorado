// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import (
	"fmt"
	"reflect"
	"time"

	"akvorado/common/helpers"
	"akvorado/inlet/metadata/provider"
	"akvorado/inlet/metadata/provider/gnmi"
	"akvorado/inlet/metadata/provider/snmp"
	"akvorado/inlet/metadata/provider/static"

	"github.com/mitchellh/mapstructure"
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

	// Provider defines the configuration of the providers to use
	Providers []ProviderConfiguration

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

// ConfigurationUnmarshallerHook renames "provider" to "providers".
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// provider → providers
		{
			var providerKey, providersKey *reflect.Value
			fromKeys := from.MapKeys()
			for i, k := range fromKeys {
				k = helpers.ElemOrIdentity(k)
				if k.Kind() != reflect.String {
					return from.Interface(), nil
				}
				if helpers.MapStructureMatchName(k.String(), "Provider") {
					providerKey = &fromKeys[i]
				} else if helpers.MapStructureMatchName(k.String(), "Providers") {
					providersKey = &fromKeys[i]
				}
			}
			if providersKey != nil && providerKey != nil {
				return nil, fmt.Errorf("cannot have both %q and %q", providerKey.String(), providersKey.String())
			}
			if providerKey != nil {
				from.SetMapIndex(reflect.ValueOf("providers"), from.MapIndex(*providerKey))
				from.SetMapIndex(*providerKey, reflect.Value{})
			}
		}
		return from.Interface(), nil
	}
}

// ProviderConfiguration represents the configuration for a metadata provider.
type ProviderConfiguration struct {
	// Config is the actual configuration for the provider.
	Config provider.Configuration
}

// MarshalYAML undoes ConfigurationUnmarshallerHook().
func (pc ProviderConfiguration) MarshalYAML() (interface{}, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(pc, providers)
}

// MarshalJSON undoes ConfigurationUnmarshallerHook().
func (pc ProviderConfiguration) MarshalJSON() ([]byte, error) {
	return helpers.ParametrizedConfigurationMarshalJSON(pc, providers)
}

var providers = map[string](func() provider.Configuration){
	"snmp":   snmp.DefaultConfiguration,
	"gnmi":   gnmi.DefaultConfiguration,
	"static": static.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(ProviderConfiguration{}, providers))
}
