// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/binary"
	"math"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder"
)

// psampField is an information element with its raw value, used in a PSAMP
// selector report.
type psampField struct {
	value []byte
	id    uint16
}

// psampMessage builds an IPFIX message with a PSAMP selector report for
// selector 17 (scope: selectorId, then the provided fields), followed by a
// data record using this selector.
func psampMessage(fields ...psampField) []byte {
	be16 := binary.BigEndian.AppendUint16
	be32 := binary.BigEndian.AppendUint32
	set := func(id uint16, content []byte) []byte {
		return append(be16(be16(nil, id), uint16(4+len(content))), content...)
	}

	optionsTemplate := be16(be16(be16(nil, 265), uint16(1+len(fields))), 1)
	optionsTemplate = be16(be16(optionsTemplate, 302), 4)
	optionsRecord := be32(nil, 17)
	for _, f := range fields {
		optionsTemplate = be16(be16(optionsTemplate, f.id), uint16(len(f.value)))
		optionsRecord = append(optionsRecord, f.value...)
	}
	template := be16(be16(nil, 256), 3)
	template = be16(be16(be16(be16(be16(be16(template, 8), 4), 12), 4), 302), 4)
	record := be32([]byte{192, 0, 2, 1, 192, 0, 2, 2}, 17)

	body := set(3, optionsTemplate)
	body = append(body, set(265, optionsRecord)...)
	body = append(body, set(2, template)...)
	body = append(body, set(256, record)...)
	msg := be16(be16(nil, 10), uint16(16+len(body)))
	msg = be32(be32(be32(msg, 1_700_000_000), 1), 0)
	return append(msg, body...)
}

func TestDecodePSAMPSamplingRate(t *testing.T) {
	u16 := func(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }
	u32 := func(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }
	algorithm := func(v uint16) psampField { return psampField{u16(v), 304} }
	cases := []struct {
		description string
		fields      []psampField
		expected    uint64
	}{
		{
			"systematic count-based",
			[]psampField{algorithm(1), {u32(1), 305}, {u32(999), 306}},
			1000,
		},
		{
			"random n-out-of-N",
			[]psampField{algorithm(3), {u32(1), 309}, {u32(10), 310}},
			10,
		},
		{
			"random n-out-of-N, reduced-size",
			[]psampField{algorithm(3), {[]byte{1}, 309}, {[]byte{10}, 310}},
			10,
		},
		{
			"uniform probabilistic",
			[]psampField{
				algorithm(4),
				{binary.BigEndian.AppendUint64(nil, math.Float64bits(0.001)), 311},
			},
			1000,
		},
		{
			"uniform probabilistic, reduced-size",
			[]psampField{algorithm(4), {u32(math.Float32bits(0.01)), 311}},
			100,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			_, nfdecoder, bf, got, finalize := setup(t, false)
			_, err := nfdecoder.Decode(decoder.RawFlow{
				Payload:      psampMessage(tc.fields...),
				Source:       netip.MustParseAddr("::ffff:127.0.0.1"),
				TimeReceived: time.Unix(1_700_000_000, 0),
			}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
			if err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}
			if len(*got) != 1 {
				t.Fatalf("Decode() returned %d flows, expected 1", len(*got))
			}
			if rate := (*got)[0].SamplingRate; rate != tc.expected {
				t.Errorf("Decode() sampling rate = %d, expected %d", rate, tc.expected)
			}
		})
	}
}
