// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package syslogcgnat

import "akvorado/inlet/flow/input"

// Configuration describes syslog CGNAT input configuration.
type Configuration struct {
	// Listen tells which address/port to listen to.
	Listen string `validate:"required,listen"`
	// ReceiveBuffer is the value of the requested receive buffer size.
	ReceiveBuffer uint
}

// DefaultConfiguration is the default configuration for this input.
func DefaultConfiguration() input.Configuration {
	return &Configuration{
		Listen: ":0",
	}
}
