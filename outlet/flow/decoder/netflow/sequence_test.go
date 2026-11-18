// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder"
)

// TestDecodeSequence decodes captures whose packets were selected from a longer
// export: the gaps between their sequence numbers are counted as missing.
// iosxe26-ipfix-sequence.pcap comes from a Cisco C8000V running IOS XE
// 26.01.01: five consecutive application table messages of 16 records each,
// with the fourth one removed. The IPFIX sequence number also counts the
// records of options templates.
func TestDecodeSequence(t *testing.T) {
	cases := []struct {
		pcap     string
		expected map[string]string
	}{
		{
			pcap:     "iosxe-v9-timestamps.pcap",
			expected: map[string]string{},
		}, {
			pcap: "nfv9.pcap",
			expected: map[string]string{
				`sequence_missing_total{exporter="::ffff:127.0.0.1",version="9"}`: "143",
			},
		}, {
			pcap: "multiplesamplingrates.pcap",
			expected: map[string]string{
				`sequence_missing_total{exporter="::ffff:127.0.0.1",version="9"}`:   "605",
				`sequence_reordered_total{exporter="::ffff:127.0.0.1",version="9"}`: "1",
			},
		}, {
			pcap: "iosxe26-ipfix-sequence.pcap",
			expected: map[string]string{
				`sequence_missing_total{exporter="::ffff:127.0.0.1",version="10"}`: "16",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.pcap, func(t *testing.T) {
			r, nfdecoder, bf, _, finalize := setup(t, false)
			for data := range helpers.ReadManyPcapL4(t, filepath.Join("testdata", tc.pcap)) {
				if _, err := nfdecoder.Decode(decoder.RawFlow{
					Payload: data,
					Source:  netip.MustParseAddr("::ffff:127.0.0.1"),
				}, decoder.Options{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize); err != nil {
					t.Fatalf("Decode() error:\n%+v", err)
				}
			}
			got := r.GetMetrics("akvorado_outlet_flow_decoder_netflow_", "sequence_")
			if diff := helpers.Diff(got, tc.expected); diff != "" {
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestSequenceTrackerObserve(t *testing.T) {
	type observation struct {
		seq, increment uint32
		known          bool
	}
	cases := []struct {
		description  string
		observations []observation
		missing      uint32
		backward     int
	}{
		{
			description:  "in order",
			observations: []observation{{10, 1, true}, {11, 1, true}, {12, 1, true}},
		}, {
			description:  "gap",
			observations: []observation{{10, 1, true}, {14, 1, true}},
			missing:      3,
		}, {
			description:  "records",
			observations: []observation{{100, 16, true}, {116, 16, true}, {148, 16, true}},
			missing:      16,
		}, {
			description:  "wraparound",
			observations: []observation{{4294967294, 1, true}, {4294967295, 1, true}, {0, 1, true}, {2, 1, true}},
			missing:      1,
		}, {
			description:  "unknown increment resynchronizes",
			observations: []observation{{100, 0, false}, {150, 10, true}, {160, 1, true}},
		}, {
			description:  "step back resynchronizes",
			observations: []observation{{1000, 1, true}, {3, 1, true}, {4, 1, true}},
			backward:     1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			var st sequenceTracker
			key := sequenceKey{exporter: "192.0.2.1", version: 10, obsDomainID: 6}
			other := sequenceKey{exporter: "192.0.2.1", version: 10, obsDomainID: 7}
			var missing uint32
			backward := 0
			for _, o := range tc.observations {
				m, b := st.observe(key, o.seq, o.increment, o.known)
				missing += m
				if b {
					backward++
				}
				// Another observation domain has its own sequence.
				if m, b := st.observe(other, 0, 0, true); m != 0 || b {
					t.Fatalf("observe() on another domain = %d, %v", m, b)
				}
			}
			if missing != tc.missing || backward != tc.backward {
				t.Errorf("observe() = %d missing, %d backward, expected %d, %d",
					missing, backward, tc.missing, tc.backward)
			}
		})
	}
}
