// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package logger

// Configuration if the configuration for logger. Currently, there is no configuration.
type Configuration struct{}

// DefaultConfiguration is the default logging configuration.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
