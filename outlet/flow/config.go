// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

// Configuration describes the configuration for the flow component.
type Configuration struct {
	// StatePersistFile defines a file to store decoder state (templates, sampling
	// rates) to survive restarts.
	StatePersistFile string `validate:"isdefault|filepath"`
}

// DefaultConfiguration returns the default configuration for the flow component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
