// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

// rawField is an information element with its raw value.
type rawField struct {
	value []byte
	id    uint16
}

// decodeIPFIXRecord decodes an IPFIX message with one template and one data
// record made of source and destination IPv4 addresses followed by the
// provided fields.
func decodeIPFIXRecord(t *testing.T, fields ...rawField) *schema.FlowMessage {
	t.Helper()
	be16 := binary.BigEndian.AppendUint16
	template := be16(be16(nil, 256), uint16(2+len(fields)))
	template = be16(be16(be16(be16(template, 8), 4), 12), 4)
	record := []byte{192, 0, 2, 1, 192, 0, 2, 2}
	for _, f := range fields {
		template = be16(be16(template, f.id), uint16(len(f.value)))
		record = append(record, f.value...)
	}
	body := append(be16(be16(nil, 2), uint16(4+len(template))), template...)
	body = append(be16(be16(body, 256), uint16(4+len(record))), record...)
	msg := be16(be16(nil, 10), uint16(16+len(body)))
	msg = binary.BigEndian.AppendUint32(msg, 1_700_000_000)
	msg = append(msg, 0, 0, 0, 1, 0, 0, 0, 0) // sequence, domain
	msg = append(msg, body...)

	_, nfdecoder, bf, got, finalize := setup(t, true)
	_, err := nfdecoder.Decode(decoder.RawFlow{
		Payload:      msg,
		Source:       netip.MustParseAddr("::ffff:127.0.0.1"),
		TimeReceived: time.Unix(1_700_000_000, 0),
	}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("Decode() returned %d flows, expected 1", len(*got))
	}
	return (*got)[0]
}

func TestDecodeNextHopUnset(t *testing.T) {
	ipNextHop := rawField{[]byte{192, 0, 2, 254}, 15}
	bgpNextHop := rawField{[]byte{0, 0, 0, 0}, 18}
	for _, fields := range [][]rawField{
		{ipNextHop, bgpNextHop},
		{bgpNextHop, ipNextHop},
	} {
		got := decodeIPFIXRecord(t, fields...).NextHop
		if expected := netip.MustParseAddr("::ffff:192.0.2.254"); got != expected {
			t.Errorf("Decode() next hop = %s, expected %s", got, expected)
		}
	}
}

func TestDecodeMPLSLabelZero(t *testing.T) {
	got := decodeIPFIXRecord(t,
		rawField{[]byte{0x00, 0x01, 0x00}, 70}, // label 16
		rawField{[]byte{0x00, 0x00, 0x01}, 71}, // label 0, bottom of stack
		rawField{[]byte{0x00, 0x00, 0x00}, 72}, // unused
	).OtherColumns[schema.ColumnMPLSLabels]
	if diff := helpers.Diff(got, []uint32{16, 0}); diff != "" {
		t.Errorf("Decode() MPLS labels (-got, +want):\n%s", diff)
	}
}

func TestDecodeTCPControlBitsDataOffset(t *testing.T) {
	// Data offset of 5 (the 4 most significant bits) with SYN and ACK.
	got := decodeIPFIXRecord(t, rawField{[]byte{0x50, 0x12}, 6}).
		OtherColumns[schema.ColumnTCPFlags]
	if diff := helpers.Diff(got, uint16(0x12)); diff != "" {
		t.Errorf("Decode() TCP flags (-got, +want):\n%s", diff)
	}
}
