// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"akvorado/common/helpers"
	"akvorado/inlet/flow/input"
)

// Configuration describes UDP input configuration.
type Configuration struct {
	// Listen tells which port to listen to.
	Listen string `validate:"required,listen"`
	// Workers define the number of workers to use for receiving flows. The max
	// should match the array length in reuseport_kern.c.
	Workers int `validate:"required,min=1,max=256"`
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
		Listen:  ":0",
		Workers: 1,
	}
}

func init() {
	helpers.RegisterMapstructureDeprecatedFields[Configuration]("QueueSize")
}
