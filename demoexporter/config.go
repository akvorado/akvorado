// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package demoexporter

// Configuration describes the configuration for the demo exporter component.
type Configuration struct {
	// Hostname is the name of the emulated exporter.
	Hostname string `validate:"required"`
}

// DefaultConfiguration represents the default configuration for the demo exporter component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
