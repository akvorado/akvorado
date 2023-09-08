// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package routing

import (
	"akvorado/common/helpers"
	"akvorado/inlet/routing/provider"
	"akvorado/inlet/routing/provider/bioris"
	"akvorado/inlet/routing/provider/bmp"
)

// Configuration describes the configuration for the routing client.
type Configuration struct {
	// Provider defines the configuration of the provider to use
	Provider ProviderConfiguration
}

// DefaultConfiguration represents the default configuration for the routing client.
func DefaultConfiguration() Configuration {
	return Configuration{}
}

// ProviderConfiguration represents the configuration for a routing provider.
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
	"bmp":    bmp.DefaultConfiguration,
	"bioris": bioris.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(ProviderConfiguration{}, providers))
}
