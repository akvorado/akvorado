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

func TestNTPToUnix(t *testing.T) {
	// RFC 7011, section 6.1.9 and 6.1.10: seconds since 1900, then a fraction.
	ntp := uint64(1_700_000_000+ntpEpochOffset)<<32 | 1<<31
	if got := ntpToUnix(ntp); got != 1_700_000_000 {
		t.Errorf("ntpToUnix() = %d, expected 1700000000", got)
	}
}

// TestDecodeFlowStartCiscoIOSXE decodes captures from a Cisco C8000V running
// IOS XE 17.18.03a with `timestamp-source: netflow-first-switched`. The
// expected bounds are the flow start times computed from the captures: with
// NetFlow v9, from FIRST_SWITCHED and the header (RFC 3954); with IPFIX, from
// flowStartMilliseconds, as the flowStartSysUpTime also exported cannot be
// converted without systemInitTimeMilliseconds.
func TestDecodeFlowStartCiscoIOSXE(t *testing.T) {
	cases := []struct {
		name     string
		min, max uint32
	}{
		{"v9", 1790709870, 1790709915},
		{"ipfix", 1790709884, 1790709917},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, nfdecoder, bf, got, finalize := setup(t, false)
			pcap := filepath.Join("testdata", fmt.Sprintf("iosxe-%s-timestamps.pcap", tc.name))
			for data := range helpers.ReadManyPcapL4(t, pcap) {
				_, err := nfdecoder.Decode(decoder.RawFlow{
					Payload: data,
					Source:  netip.MustParseAddr("::ffff:127.0.0.1"),
				}, decoder.Options{TimestampSource: pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED}, bf, finalize)
				if err != nil {
					t.Fatalf("Decode() error:\n%+v", err)
				}
			}
			if len(*got) == 0 {
				t.Fatal("Decode() returned no flow")
			}
			gotMin, gotMax := (*got)[0].TimeReceived, (*got)[0].TimeReceived
			for _, flow := range *got {
				gotMin, gotMax = min(gotMin, flow.TimeReceived), max(gotMax, flow.TimeReceived)
			}
			if gotMin != tc.min || gotMax != tc.max {
				t.Errorf("Decode() TimeReceived in [%d, %d], expected [%d, %d]",
					gotMin, gotMax, tc.min, tc.max)
			}
		})
	}
}
