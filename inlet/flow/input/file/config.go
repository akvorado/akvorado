// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package file

import "akvorado/inlet/flow/input"

// Configuration describes file input configuration.
type Configuration struct {
	// Paths to use as input
	Paths []string `validate:"min=1,dive,required"`
}

// DefaultConfiguration descrives the default configuration for file input.
func DefaultConfiguration() input.Configuration {
	return &Configuration{}
}
