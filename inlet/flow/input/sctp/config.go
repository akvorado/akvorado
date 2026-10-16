// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sctp

import (
	"akvorado/inlet/flow/input"
)

// Configuration describes SCTP input configuration.
type Configuration struct {
	// Listen tells which port to listen to.
	Listen string `validate:"required,listen"`
}

// DefaultConfiguration is the default configuration for this input
func DefaultConfiguration() input.Configuration {
	return &Configuration{
		Listen: ":0",
	}
}
