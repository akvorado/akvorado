// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

// Configuration describes the configuration for the console component.
type Configuration struct {
	// ServeLiveFS serve files from the filesystem instead of the embedded versions.
	ServeLiveFS bool
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
