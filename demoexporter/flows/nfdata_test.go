// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
	"akvorado/outlet/flow/decoder/netflow"
)

func TestGetNetFlowData(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	bf := sch.NewFlowMessage()
	nfdecoder := netflow.New(r, decoder.Dependencies{Schema: sch})

	ch := getNetFlowTemplates(
		context.Background(),
		50,
		30000,
		time.Date(2022, 3, 15, 14, 33, 0, 0, time.UTC),
		time.Date(2022, 3, 15, 15, 33, 0, 0, time.UTC))
	got := []*schema.FlowMessage{}
	finalize := func() {
		bf.TimeReceived = 0
		// Keep a copy of the current flow message
		clone := *bf
		got = append(got, &clone)
		// And clear the flow message
		bf.Clear()
	}

	for payload := range ch {
		if _, err := nfdecoder.Decode(decoder.RawFlow{
			Payload: payload, Source: netip.MustParseAddr("::ffff:127.0.0.1"),
		}, decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize); err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}

	ch = getNetFlowData(
		context.Background(),
		[]generatedFlow{
			{
				SrcAddr: net.ParseIP("192.0.2.206"),
				DstAddr: net.ParseIP("203.0.113.165"),
				EType:   0x800,
				IPFlow: IPFlow{
					Octets:        1500,
					Packets:       1,
					Proto:         6,
					SrcPort:       443,
					DstPort:       34974,
					InputInt:      10,
					OutputInt:     20,
					SrcAS:         65201,
					DstAS:         65202,
					ForwardStatus: 64,
					SrcMask:       24,
					DstMask:       23,
				},
			}, {
				SrcAddr: net.ParseIP("2001:db8::1"),
				DstAddr: net.ParseIP("2001:db8:2:0:cea5:d643:ec43:3772"),
				EType:   0x86dd,
				IPFlow: IPFlow{
					Octets:        1300,
					Packets:       1,
					Proto:         6,
					SrcPort:       33179,
					DstPort:       443,
					InputInt:      20,
					OutputInt:     10,
					SrcAS:         65201,
					DstAS:         65202,
					ForwardStatus: 64,
					SrcMask:       48,
					DstMask:       48,
				},
			}, {
				SrcAddr: net.ParseIP("192.0.2.236"),
				DstAddr: net.ParseIP("203.0.113.67"),
				EType:   0x800,
				IPFlow: IPFlow{
					Octets:        1339,
					Packets:       1,
					Proto:         6,
					SrcPort:       443,
					DstPort:       33199,
					InputInt:      10,
					OutputInt:     20,
					SrcAS:         65201,
					DstAS:         65202,
					ForwardStatus: 64,
					SrcMask:       24,
					DstMask:       24,
				},
			},
		},
		100,
		time.Date(2022, 3, 15, 14, 33, 0, 0, time.UTC),
		time.Date(2022, 3, 15, 16, 33, 0, 0, time.UTC))
	for payload := range ch {
		if _, err := nfdecoder.Decode(decoder.RawFlow{
			Payload: payload, Source: netip.MustParseAddr("::ffff:127.0.0.1"),
		}, decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}, bf, finalize); err != nil {
			t.Fatalf("Decode() error:\n%+v", err)
		}
	}
	expected := []*schema.FlowMessage{
		{
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:192.0.2.206"),
			DstAddr:         netip.MustParseAddr("::ffff:203.0.113.165"),
			InIf:            10,
			OutIf:           20,
			SrcAS:           65201,
			DstAS:           65202,
			SrcNetMask:      24,
			DstNetMask:      23,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            1500,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv4,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          34974,
				schema.ColumnForwardingStatus: 64,
			},
		},
		{
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:192.0.2.236"),
			DstAddr:         netip.MustParseAddr("::ffff:203.0.113.67"),
			InIf:            10,
			OutIf:           20,
			SrcAS:           65201,
			DstAS:           65202,
			SrcNetMask:      24,
			DstNetMask:      24,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            1339,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv4,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          33199,
				schema.ColumnForwardingStatus: 64,
			},
		},
		{
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("2001:db8::1"),
			DstAddr:         netip.MustParseAddr("2001:db8:2:0:cea5:d643:ec43:3772"),
			InIf:            20,
			OutIf:           10,
			SrcAS:           65201,
			DstAS:           65202,
			SrcNetMask:      48,
			DstNetMask:      48,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            1300,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv6,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          33179,
				schema.ColumnDstPort:          443,
				schema.ColumnForwardingStatus: 64,
			},
		},
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("getNetFlowData() (-got, +want):\n%s", diff)
	}
}
