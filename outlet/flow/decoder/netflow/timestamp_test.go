// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder"
)

func TestUptimeToUnix(t *testing.T) {
	cases := []struct {
		description         string
		export, uptime, ref uint64
		expected            uint32
	}{
		{"same time", 1_700_000_000, 1_000_000, 1_000_000, 1_700_000_000},
		{"60 s before export", 1_700_000_000, 1_000_000, 940_000, 1_699_999_940},
		{"sub-second rounds down", 1_700_000_000, 1_000_000, 999_999, 1_699_999_999},
		{"uptime wrapped", 1_700_000_000, 1_000, 1<<32 - 9_000, 1_699_999_990},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			if got := uptimeToUnix(tc.export, tc.uptime, tc.ref); got != tc.expected {
				t.Errorf("uptimeToUnix() = %d, expected %d", got, tc.expected)
			}
		})
	}
}

// ipfixField is an information element with its length and value, used to
// build an IPFIX message for tests.
type ipfixField struct {
	id     uint16
	length uint16
	value  uint64
}

// ipfixMessage builds an IPFIX message with one template and one data record
// with source and destination IPv4 addresses followed by the provided fields.
func ipfixMessage(exportTime uint32, fields ...ipfixField) []byte {
	be16 := binary.BigEndian.AppendUint16
	be32 := binary.BigEndian.AppendUint32
	template := be16(be16(nil, 256), uint16(2+len(fields)))
	template = be16(be16(be16(be16(template, 8), 4), 12), 4)
	record := []byte{192, 0, 2, 1, 192, 0, 2, 2}
	for _, f := range fields {
		template = be16(be16(template, f.id), f.length)
		value := binary.BigEndian.AppendUint64(nil, f.value)
		record = append(record, value[8-f.length:]...)
	}
	body := append(be16(be16(nil, 2), uint16(4+len(template))), template...)
	body = append(be16(be16(body, 256), uint16(4+len(record))), record...)
	msg := be16(be16(nil, 10), uint16(16+len(body)))
	msg = be32(be32(be32(msg, exportTime), 1), 0)
	return append(msg, body...)
}

func TestDecodeIPFIXFlowStart(t *testing.T) {
	const (
		export = 1_700_000_000
		start  = 1_699_999_997
		ntp    = (start + ntpEpochOffset) << 32
	)
	cases := []struct {
		description string
		fields      []ipfixField
		expected    uint32
	}{
		{"flowStartSeconds", []ipfixField{{150, 4, start}}, start},
		{"flowStartMilliseconds", []ipfixField{{152, 8, start*1000 + 500}}, start},
		{"flowStartMicroseconds", []ipfixField{{154, 8, ntp | 1<<31}}, start},
		{"flowStartNanoseconds", []ipfixField{{156, 8, ntp | 1<<31}}, start},
		{
			"flowStartSysUpTime with systemInitTimeMilliseconds",
			[]ipfixField{{160, 8, 1_699_996_400_000}, {22, 4, 3_597_000}},
			start,
		},
		{
			"systemInitTimeMilliseconds after flowStartSysUpTime",
			[]ipfixField{{22, 4, 3_597_000}, {160, 8, 1_699_996_400_000}},
			start,
		},
		{
			"flowStartSysUpTime without systemInitTimeMilliseconds",
			[]ipfixField{{22, 4, 3_597_000}},
			export + 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			_, nfdecoder, bf, got, finalize := setup(t, false)
			_, err := nfdecoder.Decode(decoder.RawFlow{
				Payload:      ipfixMessage(export, tc.fields...),
				Source:       netip.MustParseAddr("::ffff:127.0.0.1"),
				TimeReceived: time.Unix(export+1, 0),
			}, decoder.Options{TimestampSource: pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED}, bf, finalize)
			if err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}
			if len(*got) != 1 {
				t.Fatalf("Decode() returned %d flows, expected 1", len(*got))
			}
			if ts := (*got)[0].TimeReceived; ts != tc.expected {
				t.Errorf("Decode() TimeReceived = %d, expected %d", ts, tc.expected)
			}
		})
	}
}
