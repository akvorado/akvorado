// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/demoexporter/bmp"
	"akvorado/demoexporter/snmp"
)

// Configuration describes the configuration for the flows component. Flows come
// from a trained model. A mapping distributes them over a fleet of exporters
// and their interfaces, and this instance sends the ones assigned to its own
// exporter.
type Configuration struct {
	// Model is the path to a trained flow model. Empty uses the model embedded
	// in the binary.
	Model string
	// Exporter is the exporter emulated by this instance. It must be a key of
	// Exporters. The demo exporter service sets it to its hostname.
	Exporter string `mapstructure:"-" yaml:"-"`
	// Target specifies the IP address and port to send flows to.
	Target string `validate:"required,hostname_port"`
	// Seed alters the generated flows.
	Seed int64
	// Rate is the average number of flows per second for the whole fleet.
	Rate float64 `validate:"gt=0"`
	// MaxFlowsPerSecond caps the number of flows generated each second for the
	// whole fleet.
	MaxFlowsPerSecond int `validate:"min=1"`
	// LocalASN is the AS number used for the customer side of the flows.
	LocalASN uint32 `validate:"min=1"`
	// Exporters describes the whole fleet of exporters, shared by all the
	// instances.
	Exporters map[string]ExporterConfiguration `validate:"min=1,dive"`
	// Mapping distributes the flows over the fleet. The first matching rule
	// wins.
	Mapping []MappingRuleConfiguration `validate:"min=1,dive"`
}

// ExporterConfiguration describes one exporter of the fleet.
type ExporterConfiguration struct {
	// SamplingRate is the sampling rate advertised by this exporter.
	SamplingRate int `validate:"min=1"`
	// Interfaces describes the interfaces of this exporter, indexed by ifIndex.
	Interfaces map[int]InterfaceConfiguration `validate:"min=1,dive,keys,min=1,endkeys"`
}

// InterfaceConfiguration describes one interface of an exporter.
type InterfaceConfiguration struct {
	snmp.InterfaceConfiguration `mapstructure:",squash" yaml:",inline"`
	// Boundary tells if the interface faces the Internet or the inside of the
	// network.
	Boundary schema.InterfaceBoundary `validate:"required"`
	// ASN is the AS number of the neighbor behind this interface, for external
	// interfaces. It is used to rebuild AS paths for the BMP configuration.
	ASN uint32
	// Communities are attached to the routes learned from this interface in the
	// BMP configuration.
	Communities []bmp.Community
}

// MappingRuleConfiguration assigns a share of the flows to exporter interfaces.
// An empty match applies to all flows.
type MappingRuleConfiguration struct {
	// ASN restricts the rule to flows whose remote AS number is in the list.
	ASN []uint32
	// Direction restricts the rule to one flow direction.
	Direction MatchDirection
	// Assign lists the external interfaces receiving the matched flows, with
	// their weights.
	Assign []AssignConfiguration `validate:"min=1,dive"`
}

// MatchDirection restricts a mapping rule to a flow direction.
type MatchDirection int

const (
	// MatchDirectionAny matches both directions.
	MatchDirectionAny MatchDirection = iota
	// MatchDirectionInbound matches flows from the Internet.
	MatchDirectionInbound
	// MatchDirectionOutbound matches flows to the Internet.
	MatchDirectionOutbound
)

// AssignConfiguration is one target of a mapping rule.
type AssignConfiguration struct {
	// Exporter is the exporter receiving the flows.
	Exporter string `validate:"required"`
	// Interface is the ifIndex of the external interface of the exporter
	// carrying the flows.
	Interface int `validate:"min=1"`
	// Weight is the share of the rule traffic for this entry, relative to the
	// other entries of the rule.
	Weight float64 `validate:"gt=0"`
}

// DefaultConfiguration represents the default configuration for the flows
// component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Rate:              50,
		MaxFlowsPerSecond: 5000,
		LocalASN:          64496,
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(ExporterConfiguration{SamplingRate: 1000}))
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(AssignConfiguration{Weight: 1}))
}
