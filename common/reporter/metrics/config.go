// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metrics

// Configuration is currently empty as this sub-component is not
// configurable yet.
type Configuration struct{}

// DefaultConfiguration is the default metrics configuration.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
