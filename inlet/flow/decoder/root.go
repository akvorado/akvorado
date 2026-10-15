// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package decoder defines what the inlet knows about each flow decoder. The
// actual decoding happens in the outlet.
package decoder

import (
	"fmt"

	"akvorado/common/pb"
)

// Decoder is the inlet side of a flow decoder.
type Decoder interface {
	// Name returns the decoder name.
	Name() string
}

// Splitter is a decoder able to split a byte stream into messages.
type Splitter interface {
	Decoder
	// Split is a bufio.SplitFunc returning one message at a time.
	Split(data []byte, atEOF bool) (advance int, token []byte, err error)
}

type sflow struct{}

func (sflow) Name() string { return "sflow" }

type gob struct{}

func (gob) Name() string { return "gob" }

var decoders = map[pb.RawFlow_Decoder]Decoder{
	pb.RawFlow_DECODER_NETFLOW: netflow{},
	pb.RawFlow_DECODER_SFLOW:   sflow{},
	pb.RawFlow_DECODER_GOB:     gob{},
}

// Get returns the decoder matching the provided protobuf value.
func Get(d pb.RawFlow_Decoder) (Decoder, error) {
	if dec, ok := decoders[d]; ok {
		return dec, nil
	}
	return nil, fmt.Errorf("unknown decoder %s", d)
}
