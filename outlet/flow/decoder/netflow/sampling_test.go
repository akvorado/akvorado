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

var (
	be16 = binary.BigEndian.AppendUint16
	be32 = binary.BigEndian.AppendUint32
)

// flowSet builds a NetFlow v9 flow set or an IPFIX set.
func flowSet(id uint16, content []byte) []byte {
	return append(be16(be16(nil, id), uint16(4+len(content))), content...)
}

// nfv9Packet builds a NetFlow v9 packet with the provided source ID.
func nfv9Packet(sourceID uint32, sets ...[]byte) []byte {
	msg := be16(be16(nil, 9), uint16(len(sets)))
	msg = be32(be32(be32(be32(msg, 0), 1_700_000_000), 1), sourceID)
	for _, s := range sets {
		msg = append(msg, s...)
	}
	return msg
}

// ipfixPacket builds an IPFIX message with the provided observation domain.
func ipfixPacket(obsDomainID uint32, sets ...[]byte) []byte {
	var body []byte
	for _, s := range sets {
		body = append(body, s...)
	}
	msg := be16(be16(nil, 10), uint16(16+len(body)))
	msg = be32(be32(be32(msg, 1_700_000_000), 1), obsDomainID)
	return append(msg, body...)
}

// Data template 256: source and destination IPv4 addresses and the sampler
// ID (IE 48, one byte).
var (
	samplerDataTemplate = be16(be16(be16(be16(be16(be16(be16(be16(nil,
		256), 3), 8), 4), 12), 4), 48), 1)
	samplerDataRecord = []byte{192, 0, 2, 1, 192, 0, 2, 2, 1}
)

// nfv9SamplerOptions builds a NetFlow v9 options template (257) and record
// like Cisco IOS XE sampler-table: system scope, sampler ID (48, 4 bytes)
// and random interval (50, 2 bytes).
func nfv9SamplerOptions(samplerID uint32, interval uint16) [][]byte {
	template := be16(be16(be16(nil, 257), 4), 8) // scope length, option length
	template = be16(be16(template, 1), 4)        // system scope
	template = be16(be16(be16(be16(template, 48), 4), 50), 2)
	record := be16(be32(be32(nil, 0), samplerID), interval)
	return [][]byte{flowSet(1, template), flowSet(257, record)}
}

func decodeSamplingRates(t *testing.T, packets ...[]byte) []uint64 {
	t.Helper()
	_, nfdecoder, bf, got, finalize := setup(t, false)
	for _, packet := range packets {
		_, err := nfdecoder.Decode(decoder.RawFlow{
			Payload:      packet,
			Source:       netip.MustParseAddr("::ffff:127.0.0.1"),
			TimeReceived: time.Unix(1_700_000_000, 0),
		}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}
	rates := []uint64{}
	for _, flow := range *got {
		rates = append(rates, flow.SamplingRate)
	}
	return rates
}

func TestSamplingRateAcrossObservationDomains(t *testing.T) {
	data := func(sourceID uint32) []byte {
		return nfv9Packet(sourceID,
			flowSet(0, samplerDataTemplate), flowSet(256, samplerDataRecord))
	}
	options := func(sourceID uint32, interval uint16) []byte {
		return nfv9Packet(sourceID, nfv9SamplerOptions(1, interval)...)
	}
	cases := []struct {
		description string
		packets     [][]byte
		expected    uint64
	}{
		{
			"same source ID",
			[][]byte{options(256, 1000), data(256)},
			1000,
		}, {
			"options and data in different source IDs",
			[][]byte{options(6, 1000), data(256)},
			1000,
		}, {
			"same sampler in several source IDs with the same rate",
			[][]byte{options(6, 1000), options(7, 1000), data(256)},
			1000,
		}, {
			"same sampler in several source IDs with different rates",
			[][]byte{options(6, 1000), options(7, 100), data(256)},
			0,
		}, {
			"exact source ID wins over other source IDs",
			[][]byte{options(6, 1000), options(256, 100), data(256)},
			100,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			rates := decodeSamplingRates(t, tc.packets...)
			if len(rates) != 1 || rates[0] != tc.expected {
				t.Errorf("Decode() sampling rates = %v, expected [%d]", rates, tc.expected)
			}
		})
	}
}

func TestSamplingRateSelectorIDInScope(t *testing.T) {
	// PSAMP selector report (RFC 5476, section 6.5.2): the selector ID is
	// the scope, the sampling interval is not.
	optionsTemplate := be16(be16(be16(nil, 257), 3), 1) // field count, scope count
	optionsTemplate = be16(be16(be16(be16(optionsTemplate, 302), 4), 305), 4)
	optionsTemplate = be16(be16(optionsTemplate, 306), 4)
	optionsRecord := func(selectorID, space uint32) []byte {
		return be32(be32(be32(nil, selectorID), 1), space)
	}
	template := be16(be16(be16(be16(be16(be16(be16(be16(nil,
		256), 3), 8), 4), 12), 4), 302), 4)
	record := be32([]byte{192, 0, 2, 1, 192, 0, 2, 2}, 5)
	packet := ipfixPacket(0,
		flowSet(3, optionsTemplate),
		flowSet(257, append(optionsRecord(5, 999), optionsRecord(7, 99)...)),
		flowSet(2, template),
		flowSet(256, record))
	rates := decodeSamplingRates(t, packet)
	if len(rates) != 1 || rates[0] != 1000 {
		t.Errorf("Decode() sampling rates = %v, expected [1000]", rates)
	}
}
