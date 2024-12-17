// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package flow handles flow decoding from protobuf messages.
package flow

import (
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

// Component represents the flow decoder component.
type Component struct {
	r *reporter.Reporter
	d *Dependencies

	metrics struct {
		decoderStats  *reporter.CounterVec
		decoderErrors *reporter.CounterVec
	}

	// Available decoders
	decoders map[pb.RawFlow_Decoder]decoder.Decoder
}

// Dependencies are the dependencies of the flow component.
type Dependencies struct {
	Schema *schema.Component
}

// New creates a new flow component.
func New(r *reporter.Reporter, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:        r,
		d:        &dependencies,
		decoders: make(map[pb.RawFlow_Decoder]decoder.Decoder),
	}

	// Initialize available decoders
	for decoderType, decoderFunc := range availableDecoders {
		c.decoders[decoderType] = decoderFunc(r, decoder.Dependencies{Schema: c.d.Schema})
	}

	// Metrics
	c.metrics.decoderStats = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "decoder_flows_total",
			Help: "Decoder processed count.",
		},
		[]string{"name"},
	)
	c.metrics.decoderErrors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "decoder_errors_total",
			Help: "Decoder processed error count.",
		},
		[]string{"name"},
	)

	return &c, nil
}
