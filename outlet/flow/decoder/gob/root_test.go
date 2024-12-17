// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package gob

import (
	"bytes"
	"encoding/gob"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

func TestGobDecoder(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	d := New(r, decoder.Dependencies{Schema: sch})
	bf := sch.NewFlowMessage()
	got := []*schema.FlowMessage{}
	finalize := func() {
		// Keep a copy of the current flow message
		clone := *bf
		got = append(got, &clone)
		// And clear the flow message
		bf.Clear()
	}

	// Create a test FlowMessage
	originalFlow := &schema.FlowMessage{
		TimeReceived:    uint32(time.Now().Unix()),
		SamplingRate:    1000,
		ExporterAddress: netip.MustParseAddr("::ffff:192.0.2.1"),
		InIf:            10,
		OutIf:           20,
		SrcAddr:         netip.MustParseAddr("::ffff:192.0.2.100"),
		DstAddr:         netip.MustParseAddr("::ffff:192.0.2.200"),
		OtherColumns: map[schema.ColumnKey]any{
			schema.ColumnBytes:        uint64(1024),
			schema.ColumnPackets:      10,
			schema.ColumnInIfBoundary: schema.InterfaceBoundaryExternal,
			schema.ColumnExporterName: "hello",
			schema.ColumnDstNetMask:   uint32(8),
			schema.ColumnDstPort:      uint16(80),
			schema.ColumnDstASPath:    []uint32{65000, 65001, 65002},
		},
	}

	// Encode to gob
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(originalFlow); err != nil {
		t.Fatalf("gob.Encode() error: %v", err)
	}

	// Test decoding
	rawFlow := decoder.RawFlow{
		TimeReceived: time.Now(),
		Payload:      buf.Bytes(),
		Source:       netip.MustParseAddr("::ffff:192.0.2.1"),
	}

	nb, err := d.Decode(rawFlow, decoder.Option{}, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	if nb != 1 {
		t.Errorf("Decode() returned %d instead of 1", nb)
	}

	// Compare the decoded flow with the original
	if diff := helpers.Diff(got, []*schema.FlowMessage{originalFlow}); diff != "" {
		t.Errorf("decoded flow differs (-got, +want):\n%s", diff)
	}
}

func TestGobDecoderInvalidPayload(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	d := New(r, decoder.Dependencies{Schema: sch})
	bf := sch.NewFlowMessage()

	rawFlow := decoder.RawFlow{
		TimeReceived: time.Now(),
		Payload:      []byte("invalid gob data"),
		Source:       netip.MustParseAddr("::ffff:192.0.2.1"),
	}

	nb, err := d.Decode(rawFlow, decoder.Option{}, bf, func() {})
	if err == nil {
		t.Errorf("expected error for invalid payload")
	}
	if nb != 0 {
		t.Errorf("Decode() returned %d instead of 0", nb)
	}
}
