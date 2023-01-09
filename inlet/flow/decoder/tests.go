// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package decoder

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// DummyDecoder is a simple decoder producing flows from random data.
// The payload is copied in IfDescription
type DummyDecoder struct{}

// Decode returns uninteresting flow messages.
func (dc *DummyDecoder) Decode(in RawFlow) []*FlowMessage {
	return []*FlowMessage{
		{
			TimeReceived:    uint64(in.TimeReceived.UTC().Unix()),
			ExporterAddress: in.Source.To16(),
			Bytes:           uint64(len(in.Payload)),
			Packets:         1,
			InIfDescription: string(in.Payload),
		},
	}
}

// Name returns the original name.
func (dc *DummyDecoder) Name() string {
	return "dummy"
}

// DecodeMessage decodes a length-prefixed protobuf message. It assumes the
// whole buffer is used. This does not use VT functions.
func (m *FlowMessage) DecodeMessage(buf []byte) error {
	messageSize, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return protowire.ParseError(n)
	}
	buf = buf[n:]
	if uint64(len(buf)) != messageSize {
		return fmt.Errorf("input buffer is of incorrect size (%d vs %d)", len(buf), messageSize)
	}
	if err := proto.Unmarshal(buf, m); err != nil {
		return err
	}
	return nil
}
