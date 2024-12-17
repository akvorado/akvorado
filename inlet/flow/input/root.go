// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package input defines the interface of an input module for inlet.
package input

import (
	"akvorado/common/daemon"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

// Input is the interface any input should meet
type Input interface {
	// Start instructs an input to start producing flows to be sent to Kafka component.
	Start() error
	// Stop instructs the input to stop producing flows.
	Stop() error
}

// SendFunc is a function to send a flow to Kafka
type SendFunc func(exporter string, flow *pb.RawFlow)

// Configuration defines the interface to instantiate an input module from its configuration.
type Configuration interface {
	// New instantiates a new input from its configuration.
	New(r *reporter.Reporter, daemon daemon.Component, send SendFunc) (Input, error)
}
