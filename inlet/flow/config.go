// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"golang.org/x/time/rate"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/input"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

// Configuration describes the configuration for the flow component
type Configuration struct {
	// Inputs define a list of input modules to enable
	Inputs []InputConfiguration `validate:"dive"`
	// RateLimit defines a rate limit on the number of flows per
	// second. The limit is per-exporter.
	RateLimit rate.Limit `validate:"isdefault|min=100"`
}

// DefaultConfiguration represents the default configuration for the flow component
func DefaultConfiguration() Configuration {
	return Configuration{
		Inputs: []InputConfiguration{{
			Decoder: "netflow",
			Config:  udp.DefaultConfiguration(),
		}, {
			Decoder: "sflow",
			Config:  udp.DefaultConfiguration(),
		}},
	}
}

// InputConfiguration represents the configuration for an input.
type InputConfiguration struct {
	// Decoder is the decoder to associate to the input.
	Decoder string
	// UseSrcAddrForExporterAddr replaces the exporter address by the transport
	// source address.
	UseSrcAddrForExporterAddr bool
	// Config is the actual configuration of the input.
	Config input.Configuration
}

// MarshalYAML undoes ConfigurationUnmarshallerHook().
func (ic InputConfiguration) MarshalYAML() (interface{}, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(ic, inputs)
}

// MarshalJSON undoes ConfigurationUnmarshallerHook().
func (ic InputConfiguration) MarshalJSON() ([]byte, error) {
	return helpers.ParametrizedConfigurationMarshalJSON(ic, inputs)
}

var inputs = map[string](func() input.Configuration){
	"udp":  udp.DefaultConfiguration,
	"file": file.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(InputConfiguration{}, inputs))
}
