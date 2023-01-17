// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package decoder

import (
	"net/netip"

	"akvorado/common/schema"
)

// DummyDecoder is a simple decoder producing flows from random data.
// The payload is copied in IfDescription
type DummyDecoder struct{}

// Decode returns uninteresting flow messages.
func (dc *DummyDecoder) Decode(in RawFlow) []*schema.FlowMessage {
	exporterAddress, _ := netip.AddrFromSlice(in.Source.To16())
	f := &schema.FlowMessage{
		TimeReceived:    uint64(in.TimeReceived.UTC().Unix()),
		ExporterAddress: exporterAddress,
	}
	schema.Flows.ProtobufAppendVarint(f, schema.ColumnBytes, uint64(len(in.Payload)))
	schema.Flows.ProtobufAppendVarint(f, schema.ColumnPackets, 1)
	schema.Flows.ProtobufAppendBytes(f, schema.ColumnInIfDescription, in.Payload)
	return []*schema.FlowMessage{f}
}

// Name returns the original name.
func (dc *DummyDecoder) Name() string {
	return "dummy"
}
