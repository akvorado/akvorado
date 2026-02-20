// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/inlet/flow/input"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

// Configuration describes the configuration for the flow component
type Configuration struct {
	// Inputs define a list of input modules to enable
	Inputs []InputConfiguration `validate:"dive"`
}

// DefaultConfiguration represents the default configuration for the flow component
func DefaultConfiguration() Configuration {
	return Configuration{
		Inputs: []InputConfiguration{{
			TimestampSource: pb.RawFlow_TS_INPUT,
			Decoder:         pb.RawFlow_DECODER_NETFLOW,
			Config:          udp.DefaultConfiguration(),
		}, {
			TimestampSource: pb.RawFlow_TS_INPUT,
			Decoder:         pb.RawFlow_DECODER_SFLOW,
			Config:          udp.DefaultConfiguration(),
		}},
	}
}

// InputConfiguration represents the configuration for an input.
type InputConfiguration struct {
	// Decoder is the decoder to associate to the input.
	Decoder pb.RawFlow_Decoder `validate:"required"`
	// UseSrcAddrForExporterAddr replaces the exporter address by the transport
	// source address.
	UseSrcAddrForExporterAddr bool
	// TimestampSource identifies the source to use to timestamp the flows
	TimestampSource pb.RawFlow_TimestampSource
	// DecapsulationProtocol is the protocol to decap. Packets not matching this protocol will be discarded.
	DecapsulationProtocol pb.RawFlow_DecapsulationProtocol
	// RateLimit is the maximum number of flows per second per exporter. 0 means no limit.
	RateLimit uint64
	// Config is the actual configuration of the input.
	Config input.Configuration
}

// MarshalYAML undoes ConfigurationUnmarshallerHook().
func (ic InputConfiguration) MarshalYAML() (any, error) {
	return helpers.ParametrizedConfigurationMarshalYAML(ic, inputs)
}

var inputs = map[string](func() input.Configuration){
	"udp":  udp.DefaultConfiguration,
	"file": file.DefaultConfiguration,
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.ParametrizedConfigurationUnmarshallerHook(InputConfiguration{}, inputs))
}
