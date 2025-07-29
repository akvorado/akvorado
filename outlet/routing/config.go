// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package routing

import (
	"akvorado/common/helpers"
	"akvorado/outlet/routing/provider"
	"akvorado/outlet/routing/provider/bioris"
	"akvorado/outlet/routing/provider/bmp"
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
func (pc ProviderConfiguration) MarshalYAML() (any, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(pc, providers)
}

var providers = map[string](func() provider.Configuration){
	"bmp":    bmp.DefaultConfiguration,
	"bioris": bioris.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(ProviderConfiguration{}, providers))
}
