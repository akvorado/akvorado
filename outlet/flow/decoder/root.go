// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package decoder handles the protocol-independent part of flow
// decoding.
package decoder

import (
	"net/netip"
	"time"

	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Decoder is the interface each decoder should implement.
type Decoder interface {
	// Decoder takes a raw flow and options. It should enqueue new flows in the
	// provided flow message. When a flow is enqueued, it will call the finalize
	// function. On error, the caller is not expected to do any cleanup.
	// Therefore, the decoder should either not raise errors once flows are
	// being built or it should do the cleanup itself (by calling `Undo()`).
	Decode(in RawFlow, options Options, bf *schema.FlowMessage, finalize FinalizeFlowFunc) (int, error)

	// Name returns the decoder name
	Name() string
}

// Options specifies option to influence the behaviour of the decoder
type Options struct {
	// TimestampSource is a selector for how to set the TimeReceived.
	TimestampSource pb.RawFlow_TimestampSource
}

// Dependencies are the dependencies for the decoder
type Dependencies struct {
	Schema *schema.Component
}

// RawFlow is an undecoded flow.
type RawFlow struct {
	TimeReceived time.Time
	Payload      []byte
	Source       netip.Addr
}

// NewDecoderFunc is the signature of a function to instantiate a decoder.
type NewDecoderFunc func(*reporter.Reporter, Dependencies) Decoder

// FinalizeFlowFunc is the signature of a function to finalize a flow. The
// caller has a reference to the flow message he provided.
type FinalizeFlowFunc func()
