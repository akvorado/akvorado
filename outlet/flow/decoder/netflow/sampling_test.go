// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder"

	"github.com/netsampler/goflow2/v3/decoders/netflow"
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

// samplingEvent is a template or a sampler option received from an exporter.
type samplingEvent struct {
	obsDomainID  uint32
	dataTemplate bool   // a data template, otherwise a sampler option
	samplerID    uint64 // for a sampler option
	rate         uint32 // for a sampler option
}

func applySamplingEvents(t *testing.T, tao *templatesAndOptions, events []samplingEvent) {
	t.Helper()
	for _, event := range events {
		if event.dataTemplate {
			if _, err := tao.AddTemplate(netflow.FlowContext{}, 9, event.obsDomainID, 256, netflow.TemplateRecord{TemplateId: 256}); err != nil {
				t.Fatalf("AddTemplate() error:\n%+v", err)
			}
			continue
		}
		tao.SetSamplingRate(9, event.obsDomainID, event.samplerID, event.rate)
	}
}

func TestGetSamplingRateAcrossObservationDomains(t *testing.T) {
	cases := []struct {
		description string
		events      []samplingEvent
		obsDomainID uint32
		expected    uint32
	}{
		{
			description: "same observation domain",
			events:      []samplingEvent{{obsDomainID: 256, dataTemplate: true}, {obsDomainID: 256, samplerID: 1, rate: 100}},
			obsDomainID: 256,
			expected:    100,
		}, {
			description: "observation domain without data template",
			events:      []samplingEvent{{obsDomainID: 6, samplerID: 1, rate: 1000}, {obsDomainID: 256, dataTemplate: true}},
			obsDomainID: 256,
			expected:    1000,
		}, {
			description: "another sampler",
			events:      []samplingEvent{{obsDomainID: 6, samplerID: 2, rate: 1000}},
			obsDomainID: 256,
			expected:    0,
		}, {
			description: "exact observation domain wins",
			events: []samplingEvent{
				{obsDomainID: 6, samplerID: 1, rate: 1000},
				{obsDomainID: 256, dataTemplate: true},
				{obsDomainID: 256, samplerID: 1, rate: 100},
			},
			obsDomainID: 256,
			expected:    100,
		}, {
			// The sampling rate may change: the last one wins.
			description: "sampling rate change",
			events:      []samplingEvent{{obsDomainID: 6, samplerID: 1, rate: 1000}, {obsDomainID: 6, samplerID: 1, rate: 100}},
			obsDomainID: 256,
			expected:    100,
		}, {
			// Another domain with data has its own sampler: ours, at 1/10000,
			// may come later.
			description: "observation domain with data template",
			events:      []samplingEvent{{obsDomainID: 7, dataTemplate: true}, {obsDomainID: 7, samplerID: 1, rate: 1000}},
			obsDomainID: 256,
			expected:    0,
		}, {
			description: "data template after the sampler option",
			events:      []samplingEvent{{obsDomainID: 7, samplerID: 1, rate: 1000}, {obsDomainID: 7, dataTemplate: true}},
			obsDomainID: 256,
			expected:    0,
		}, {
			description: "tombstone is kept",
			events: []samplingEvent{
				{obsDomainID: 7, dataTemplate: true},
				{obsDomainID: 7, samplerID: 1, rate: 1000},
				{obsDomainID: 6, samplerID: 1, rate: 1000},
			},
			obsDomainID: 256,
			expected:    0,
		}, {
			description: "observation domain 0 with data and options",
			events:      []samplingEvent{{obsDomainID: 0, dataTemplate: true}, {obsDomainID: 0, samplerID: 1, rate: 100}},
			obsDomainID: 0,
			expected:    100,
		}, {
			description: "observation domain 0 does not lend its rate",
			events:      []samplingEvent{{obsDomainID: 0, dataTemplate: true}, {obsDomainID: 0, samplerID: 1, rate: 100}},
			obsDomainID: 256,
			expected:    0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			_, nfdecoder, _, _, _ := setup(t, false)
			tao := nfdecoder.(*Decoder).collection.Get("::ffff:127.0.0.1")
			applySamplingEvents(t, tao, tc.events)
			if got := tao.GetSamplingRate(9, tc.obsDomainID, 1); got != tc.expected {
				t.Errorf("GetSamplingRate() = %d, expected %d", got, tc.expected)
			}
		})
	}
}

func TestGetSamplingRateAfterRestore(t *testing.T) {
	_, nfdecoder, _, _, _ := setup(t, false)
	applySamplingEvents(t, nfdecoder.(*Decoder).collection.Get("::ffff:127.0.0.1"), []samplingEvent{
		{obsDomainID: 6, samplerID: 1, rate: 1000},
		{obsDomainID: 7, dataTemplate: true},
		{obsDomainID: 7, samplerID: 2, rate: 10},
		{obsDomainID: 256, dataTemplate: true},
	})
	state, err := json.Marshal(nfdecoder)
	if err != nil {
		t.Fatalf("Marshal() error:\n%+v", err)
	}
	_, restored, _, _, _ := setup(t, false)
	if err := json.Unmarshal(state, restored); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	tao := restored.(*Decoder).collection.Get("::ffff:127.0.0.1")
	got := []uint32{tao.GetSamplingRate(9, 256, 1), tao.GetSamplingRate(9, 256, 2), tao.GetSamplingRate(9, 7, 2)}
	if diff := helpers.Diff(got, []uint32{1000, 0, 10}); diff != "" {
		t.Fatalf("GetSamplingRate() after restore (-got, +want):\n%s", diff)
	}
}
