// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

// Configuration describes the configuration for the SNMP component.
type Configuration struct {
	// Name is the system name of the exporter.
	Name string `mapstructure:"-" yaml:"-"`
	// Interfaces describe the interfaces attached to the exporter, indexed by
	// ifIndex. The demo exporter service takes them from the flows component.
	Interfaces map[int]InterfaceConfiguration `mapstructure:"-" yaml:"-"`
	// Listen specify the IP address the SNMP server should be bound to.
	Listen string `validate:"required,listen"`
}

// InterfaceConfiguration describes one interface of the exporter.
type InterfaceConfiguration struct {
	// Name is the interface name.
	Name string
	// Description is the interface description.
	Description string `validate:"required,min=3"`
	// Speed is the interface speed in Mbps.
	Speed uint
}

// DefaultConfiguration represents the default configuration for the SNMP component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen: ":161",
	}
}
