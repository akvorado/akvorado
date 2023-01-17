// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package decoder handles the protocol-independent part of flow
// decoding.
package decoder

import (
	"net"
	"time"

	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Decoder is the interface each decoder should implement.
type Decoder interface {
	// Decoder takes a raw flow and returns a
	// slice of flow messages. Returning nil means there was an
	// error during decoding.
	Decode(in RawFlow) []*schema.FlowMessage

	// Name returns the decoder name
	Name() string
}

// RawFlow is an undecoded flow.
type RawFlow struct {
	TimeReceived time.Time
	Payload      []byte
	Source       net.IP
}

// NewDecoderFunc is the signature of a function to instantiate a decoder.
type NewDecoderFunc func(*reporter.Reporter) Decoder
