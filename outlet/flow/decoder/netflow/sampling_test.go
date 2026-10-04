// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder"
)

// TestSamplingRateCiscoIOSXE decodes captures from a Cisco C8000V running IOS
// XE 17.18.03a with random 1-out-of-1000 sampling. The sampler options are
// exported with source ID 6, the flows with another source ID. With IPFIX,
// the sampler ID is the scope of the sampler options.
func TestSamplingRateCiscoIOSXE(t *testing.T) {
	for _, format := range []string{"v9", "ipfix"} {
		t.Run(format, func(t *testing.T) {
			_, nfdecoder, bf, got, finalize := setup(t, false)
			pcap := filepath.Join("testdata", fmt.Sprintf("iosxe-%s-sampling.pcap", format))
			for data := range helpers.ReadManyPcapL4(t, pcap) {
				_, err := nfdecoder.Decode(decoder.RawFlow{
					Payload: data,
					Source:  netip.MustParseAddr("::ffff:127.0.0.1"),
				}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
				if err != nil {
					t.Fatalf("Decode() error:\n%+v", err)
				}
			}
			rates := map[uint64]int{}
			for _, flow := range *got {
				rates[flow.SamplingRate]++
			}
			if diff := helpers.Diff(rates, map[uint64]int{1000: 18}); diff != "" {
				t.Fatalf("Decode() sampling rates (-got, +want):\n%s", diff)
			}
		})
	}
}

// TestSamplingRateTwoSamplersCiscoIOSXE decodes a capture from a Cisco C8000V
// running IOS XE 26.01.01 with two random samplers on one exporter, 1 out of
// 10 and 1 out of 100. Both sampler options come with source ID 6 and the
// flows with source ID 256: each flow gets the rate of its own sampler ID.
func TestSamplingRateTwoSamplersCiscoIOSXE(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, false)
	for data := range helpers.ReadManyPcapL4(t, filepath.Join("testdata", "iosxe26-v9-samplers.pcap")) {
		_, err := nfdecoder.Decode(decoder.RawFlow{
			Payload: data,
			Source:  netip.MustParseAddr("::ffff:127.0.0.1"),
		}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}
	rates := map[uint64]int{}
	for _, flow := range *got {
		rates[flow.SamplingRate]++
	}
	if diff := helpers.Diff(rates, map[uint64]int{10: 9, 100: 2}); diff != "" {
		t.Fatalf("Decode() sampling rates (-got, +want):\n%s", diff)
	}
}

func TestGetSamplingRateAcrossObservationDomains(t *testing.T) {
	cases := []struct {
		rates       map[samplingRateKey]uint32
		dataDomains []uint32
		description string
		expected    uint32
	}{
		{
			description: "same observation domain",
			rates:       map[samplingRateKey]uint32{{9, 256, 1}: 1000},
			expected:    1000,
		}, {
			description: "another observation domain",
			rates:       map[samplingRateKey]uint32{{9, 6, 1}: 1000},
			expected:    1000,
		}, {
			description: "several observation domains with the same rate",
			rates:       map[samplingRateKey]uint32{{9, 6, 1}: 1000, {9, 7, 1}: 1000},
			expected:    1000,
		}, {
			description: "several observation domains with different rates",
			rates:       map[samplingRateKey]uint32{{9, 6, 1}: 1000, {9, 7, 1}: 100},
			expected:    0,
		}, {
			description: "exact observation domain wins",
			rates:       map[samplingRateKey]uint32{{9, 6, 1}: 1000, {9, 256, 1}: 100},
			expected:    100,
		}, {
			description: "another sampler",
			rates:       map[samplingRateKey]uint32{{9, 6, 2}: 1000},
			expected:    0,
		}, {
			// Another domain with its own data: its sampler options are not
			// ours, ours may simply not be received yet.
			description: "another observation domain sending data",
			rates:       map[samplingRateKey]uint32{{9, 7, 1}: 1000},
			dataDomains: []uint32{7, 256},
			expected:    0,
		}, {
			description: "options-only domain next to a data domain",
			rates:       map[samplingRateKey]uint32{{9, 6, 1}: 1000, {9, 7, 1}: 10000},
			dataDomains: []uint32{7, 256},
			expected:    1000,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			tao := templatesAndOptions{SamplingRates: tc.rates}
			for _, domain := range tc.dataDomains {
				tao.markDataDomain(9, domain)
			}
			if got := tao.GetSamplingRate(9, 256, 1); got != tc.expected {
				t.Errorf("GetSamplingRate() = %d, expected %d", got, tc.expected)
			}
		})
	}
}
