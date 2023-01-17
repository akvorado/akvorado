// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package input defines the interface of an input module for inlet.
package input

import (
	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
)

// Input is the interface any input should meet
type Input interface {
	// Start instructs an input to start producing flows on the returned channel.
	Start() (<-chan []*schema.FlowMessage, error)
	// Stop instructs the input to stop producing flows.
	Stop() error
}

// Configuration the interface for the configuration for an input module.
type Configuration interface {
	// New instantiantes a new input from its configuration.
	New(r *reporter.Reporter, daemon daemon.Component, dec decoder.Decoder) (Input, error)
}
