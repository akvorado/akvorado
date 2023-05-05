// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

// Configuration describes the configuration for the SNMP component.
type Configuration struct {
	// Name defines the system name for the exporter (for SNMP)
	Name string `validate:"required"`
	// Interfaces describe the interfaces attached to the
	// exporter. This is a mapping from ifIndex to their
	// description.
	Interfaces map[int]string `validate:"min=1,dive,keys,min=1,endkeys,min=3"`
	// Listen specify the IP address the SNMP server should be bound to.
	Listen string `validate:"required,listen"`
}

// DefaultConfiguration represents the default configuration for the SNMP component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen: ":161",
	}
}
