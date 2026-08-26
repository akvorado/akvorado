// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/outlet/flow/decoder"

	"github.com/netsampler/goflow2/v3/decoders/netflow"
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
			got := uptimeToUnix(tc.export, tc.uptime, tc.ref)
			if diff := helpers.Diff(got, tc.expected); diff != "" {
				t.Errorf("uptimeToUnix() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestNTPToUnix(t *testing.T) {
	// RFC 7011, section 6.1.9 and 6.1.10: seconds since 1900, then a fraction.
	ntp := uint64(1_700_000_000+ntpEpochOffset)<<32 | 1<<31
	if diff := helpers.Diff(ntpToUnix(ntp), uint32(1_700_000_000)); diff != "" {
		t.Errorf("ntpToUnix() (-got, +want):\n%s", diff)
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
		// For ipfix, we can get the values directly from tshark:
		// tshark -r iosxe-ipfix-timestamps.pcap -d udp.port==2057,cflow -T fields -E aggregator=' ' -e cflow.abstimestart | xargs -n1 date +%s -d | sort -n
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

// TestIPFIXFlowStartSysUpTime checks that flowStartSysUpTime is converted with
// systemInitTimeMilliseconds whatever the order of the two fields.
func TestIPFIXFlowStartSysUpTime(t *testing.T) {
	start := netflow.DataField{
		Type:  netflow.IPFIX_FIELD_flowStartSysUpTime,
		Value: binary.BigEndian.AppendUint32(nil, 5_000),
	}
	systemInit := netflow.DataField{
		Type:  netflow.IPFIX_FIELD_systemInitTimeMilliseconds,
		Value: binary.BigEndian.AppendUint64(nil, 1_700_000_000_000),
	}
	for _, tc := range []struct {
		description string
		fields      []netflow.DataField
		expected    uint32
	}{
		{"system init first", []netflow.DataField{systemInit, start}, 1_700_000_005},
		{"system init last", []netflow.DataField{start, systemInit}, 1_700_000_005},
		{"no system init", []netflow.DataField{start}, 0},
	} {
		t.Run(tc.description, func(t *testing.T) {
			_, nfdecoder, bf, got, finalize := setup(t, false)
			nd := nfdecoder.(*Decoder)
			nd.decodeRecord(10, 1, nd.collection.Get("test"), tc.fields, 1_700_000_100, 0,
				decoder.Options{TimestampSource: pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED}, "test", bf, finalize)
			if len(*got) != 1 {
				t.Fatalf("decodeRecord() returned %d flows, expected 1", len(*got))
			}
			if diff := helpers.Diff((*got)[0].TimeReceived, tc.expected); diff != "" {
				t.Errorf("decodeRecord() TimeReceived (-got, +want):\n%s", diff)
			}
		})
	}
}
