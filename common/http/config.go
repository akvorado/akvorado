// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package http

// Configuration describes the configuration for the HTTP server.
type Configuration struct {
	// Listen defines the listening string to listen to.
	Listen string `validate:"listen"`
	// Profiler enables Go profiler as /debug
	Profiler bool
}

// DefaultConfiguration is the default configuration of the HTTP server.
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen: "0.0.0.0:8080",
	}
}
