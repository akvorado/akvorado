// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import "akvorado/inlet/flow/input"

// Configuration describes UDP input configuration.
type Configuration struct {
	// Listen tells which port to listen to.
	Listen string `validate:"required,listen"`
	// Workers define the number of workers to use for receiving flows.
	Workers int `validate:"required,min=1"`
	// QueueSize defines the size of the channel used to
	// communicate incoming flows. 0 can be used to disable
	// buffering.
	QueueSize uint
	// ReceiveBuffer is the value of the requested buffer size for
	// each listening socket. When 0, the value is left to the
	// default value set by the kernel (net.core.wmem_default).
	// The value cannot exceed the kernel max value
	// (net.core.wmem_max).
	ReceiveBuffer uint
}

// DefaultConfiguration is the default configuration for this input
func DefaultConfiguration() input.Configuration {
	return &Configuration{
		Listen:    ":0",
		Workers:   1,
		QueueSize: 100000,
	}
}
