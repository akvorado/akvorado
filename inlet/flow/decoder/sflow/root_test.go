// SPDX-FileCopyrightText: 2022 Tchadel Icard
// SPDX-License-Identifier: AGPL-3.0-only

package sflow

import (
	"net"
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
)

func TestDecode(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()}, decoder.Option{})

	// Send data
	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-1140.pcap"))
	got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on data")
	}
	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    1024,
			InIf:            27,
			OutIf:           28,
			SrcVlan:         100,
			DstVlan:         100,
			SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
			DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:         1500,
				schema.ColumnPackets:       1,
				schema.ColumnEType:         helpers.ETypeIPv6,
				schema.ColumnProto:         6,
				schema.ColumnSrcPort:       46026,
				schema.ColumnDstPort:       22,
				schema.ColumnSrcMAC:        40057391053392,
				schema.ColumnDstMAC:        40057381862408,
				schema.ColumnIPTTL:         64,
				schema.ColumnIPTos:         0x8,
				schema.ColumnIPv6FlowLabel: 0x68094,
				schema.ColumnTCPFlags:      0x10,
			},
		}, {
			SamplingRate:    1024,
			SrcAddr:         netip.MustParseAddr("::ffff:104.26.8.24"),
			DstAddr:         netip.MustParseAddr("::ffff:45.90.161.46"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			NextHop:         netip.MustParseAddr("::ffff:45.90.161.46"),
			InIf:            49001,
			OutIf:           25,
			DstVlan:         100,
			SrcAS:           13335,
			DstAS:           39421,
			SrcNetMask:      20,
			DstNetMask:      27,
			GotASPath:       true,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:        421,
				schema.ColumnPackets:      1,
				schema.ColumnEType:        helpers.ETypeIPv4,
				schema.ColumnProto:        6,
				schema.ColumnSrcPort:      443,
				schema.ColumnDstPort:      56876,
				schema.ColumnSrcMAC:       216372595274807,
				schema.ColumnDstMAC:       191421060163210,
				schema.ColumnIPFragmentID: 0xa572,
				schema.ColumnIPTTL:        59,
				schema.ColumnTCPFlags:     0x18,
			},
		}, {
			SamplingRate:    1024,
			SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
			DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			InIf:            27,
			OutIf:           28,
			SrcVlan:         100,
			DstVlan:         100,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:         1500,
				schema.ColumnPackets:       1,
				schema.ColumnEType:         helpers.ETypeIPv6,
				schema.ColumnProto:         6,
				schema.ColumnSrcPort:       46026,
				schema.ColumnDstPort:       22,
				schema.ColumnSrcMAC:        40057391053392,
				schema.ColumnDstMAC:        40057381862408,
				schema.ColumnIPTTL:         64,
				schema.ColumnIPTos:         0x8,
				schema.ColumnIPv6FlowLabel: 0x68094,
				schema.ColumnTCPFlags:      0x10,
			},
		}, {
			SamplingRate:    1024,
			InIf:            28,
			OutIf:           49001,
			SrcVlan:         100,
			SrcAS:           39421,
			DstAS:           26615,
			SrcAddr:         netip.MustParseAddr("::ffff:45.90.161.148"),
			DstAddr:         netip.MustParseAddr("::ffff:191.87.91.27"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			NextHop:         netip.MustParseAddr("::ffff:31.14.69.110"),
			SrcNetMask:      27,
			DstNetMask:      17,
			GotASPath:       true,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:        40,
				schema.ColumnPackets:      1,
				schema.ColumnEType:        helpers.ETypeIPv4,
				schema.ColumnProto:        6,
				schema.ColumnSrcPort:      55658,
				schema.ColumnDstPort:      5555,
				schema.ColumnSrcMAC:       138617863011056,
				schema.ColumnDstMAC:       216372595274807,
				schema.ColumnDstASPath:    []uint32{203698, 6762, 26615},
				schema.ColumnIPFragmentID: 0xd431,
				schema.ColumnIPTTL:        255,
				schema.ColumnTCPFlags:     0x2,
			},
		}, {
			SamplingRate:    1024,
			SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
			DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			InIf:            27,
			OutIf:           28,
			SrcVlan:         100,
			DstVlan:         100,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:         1500,
				schema.ColumnPackets:       1,
				schema.ColumnEType:         helpers.ETypeIPv6,
				schema.ColumnProto:         6,
				schema.ColumnSrcPort:       46026,
				schema.ColumnDstPort:       22,
				schema.ColumnSrcMAC:        40057391053392,
				schema.ColumnDstMAC:        40057381862408,
				schema.ColumnIPTTL:         64,
				schema.ColumnIPTos:         0x8,
				schema.ColumnIPv6FlowLabel: 0x68094,
				schema.ColumnTCPFlags:      0x10,
			},
		},
	}
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
	gotMetrics := r.GetMetrics(
		"akvorado_inlet_flow_decoder_sflow_",
		"flows_total",
		"sample_",
	)
	expectedMetrics := map[string]string{
		`flows_total{agent="172.16.0.3",exporter="127.0.0.1",version="5"}`:                          "1",
		`sample_records_sum{agent="172.16.0.3",exporter="127.0.0.1",type="FlowSample",version="5"}`: "14",
		`sample_sum{agent="172.16.0.3",exporter="127.0.0.1",type="FlowSample",version="5"}`:         "5",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestDecodeInterface(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t)}, decoder.Option{})

	t.Run("local interface", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-local-interface.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1024,
				SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
				DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
				ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
				InIf:            27,
				OutIf:           0, // local interface
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:   1500,
					schema.ColumnPackets: 1,
					schema.ColumnEType:   helpers.ETypeIPv6,
					schema.ColumnProto:   6,
					schema.ColumnSrcPort: 46026,
					schema.ColumnDstPort: 22,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("discard interface", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-discard-interface.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1024,
				SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
				DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
				ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
				InIf:            27,
				OutIf:           0, // discard interface
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:            1500,
					schema.ColumnPackets:          1,
					schema.ColumnEType:            helpers.ETypeIPv6,
					schema.ColumnProto:            6,
					schema.ColumnSrcPort:          46026,
					schema.ColumnDstPort:          22,
					schema.ColumnForwardingStatus: 128,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("multiple interfaces", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-multiple-interfaces.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1024,
				SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
				DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
				ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
				InIf:            27,
				OutIf:           0, // multiple interfaces
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:   1500,
					schema.ColumnPackets: 1,
					schema.ColumnEType:   helpers.ETypeIPv6,
					schema.ColumnProto:   6,
					schema.ColumnSrcPort: 46026,
					schema.ColumnDstPort: 22,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})
}

func TestDecodeSamples(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()}, decoder.Option{})

	t.Run("expanded flow sample", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-sflow-expanded-sample.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1000,
				InIf:            29001,
				OutIf:           1285816721,
				SrcAddr:         netip.MustParseAddr("::ffff:52.52.52.52"),
				DstAddr:         netip.MustParseAddr("::ffff:53.53.53.53"),
				ExporterAddress: netip.MustParseAddr("::ffff:49.49.49.49"),
				NextHop:         netip.MustParseAddr("::ffff:54.54.54.54"),
				SrcAS:           203476,
				DstAS:           203361,
				SrcVlan:         809,
				GotASPath:       true,
				SrcNetMask:      32,
				DstNetMask:      22,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:        104,
					schema.ColumnPackets:      1,
					schema.ColumnEType:        helpers.ETypeIPv4,
					schema.ColumnProto:        6,
					schema.ColumnSrcPort:      22,
					schema.ColumnDstPort:      52237,
					schema.ColumnDstASPath:    []uint32{8218, 29605, 203361},
					schema.ColumnTCPFlags:     0x18,
					schema.ColumnIPFragmentID: 0xab4e,
					schema.ColumnIPTTL:        61,
					schema.ColumnIPTos:        0x8,
					schema.ColumnSrcMAC:       0x948ed30a713b,
					schema.ColumnDstMAC:       0x22421f4a9fcd,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("flow sample with IPv4 data", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-sflow-ipv4-data.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    256,
				InIf:            0,
				OutIf:           182,
				DstVlan:         3001,
				SrcAddr:         netip.MustParseAddr("::ffff:50.50.50.50"),
				DstAddr:         netip.MustParseAddr("::ffff:51.51.51.51"),
				ExporterAddress: netip.MustParseAddr("::ffff:49.49.49.49"),
				GotASPath:       false,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:        1344,
					schema.ColumnPackets:      1,
					schema.ColumnEType:        helpers.ETypeIPv4,
					schema.ColumnProto:        17,
					schema.ColumnSrcPort:      46622,
					schema.ColumnDstPort:      58631,
					schema.ColumnSrcMAC:       1094287164743,
					schema.ColumnDstMAC:       1101091482116,
					schema.ColumnIPFragmentID: 41647,
					schema.ColumnIPTTL:        64,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("flow sample with IPv4 raw packet", func(t *testing.T) {
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-sflow-raw-ipv4.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1,
				InIf:            0,
				OutIf:           2,
				SrcAddr:         netip.MustParseAddr("::ffff:69.58.92.107"),
				DstAddr:         netip.MustParseAddr("::ffff:92.222.186.1"),
				ExporterAddress: netip.MustParseAddr("::ffff:172.19.64.116"),
				GotASPath:       false,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:        32,
					schema.ColumnPackets:      1,
					schema.ColumnEType:        helpers.ETypeIPv4,
					schema.ColumnProto:        1,
					schema.ColumnIPFragmentID: 4329,
					schema.ColumnIPTTL:        64,
					schema.ColumnIPTos:        8,
				},
			}, {
				SamplingRate:    1,
				InIf:            0,
				OutIf:           2,
				SrcAddr:         netip.MustParseAddr("::ffff:69.58.92.107"),
				DstAddr:         netip.MustParseAddr("::ffff:92.222.184.1"),
				ExporterAddress: netip.MustParseAddr("::ffff:172.19.64.116"),
				GotASPath:       false,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:        32,
					schema.ColumnPackets:      1,
					schema.ColumnEType:        helpers.ETypeIPv4,
					schema.ColumnProto:        1,
					schema.ColumnIPFragmentID: 62945,
					schema.ColumnIPTTL:        64,
					schema.ColumnIPTos:        8,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("flow sample with ICMPv4", func(t *testing.T) {
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-icmpv4.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1,
				SrcAddr:         netip.MustParseAddr("::ffff:203.0.113.4"),
				DstAddr:         netip.MustParseAddr("::ffff:203.0.113.5"),
				ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
				GotASPath:       false,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:      84,
					schema.ColumnPackets:    1,
					schema.ColumnEType:      helpers.ETypeIPv4,
					schema.ColumnProto:      1,
					schema.ColumnDstMAC:     0xd25b45ee5ecf,
					schema.ColumnSrcMAC:     0xe2efc68f8cd4,
					schema.ColumnICMPv4Type: 8,
					// schema.ColumnICMPv4Code:   0,
					schema.ColumnIPTTL:        64,
					schema.ColumnIPFragmentID: 0x90c5,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})

	t.Run("flow sample with ICMPv6", func(t *testing.T) {
		data := helpers.ReadPcapL4(t, filepath.Join("testdata", "data-icmpv6.pcap"))
		got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
		if got == nil {
			t.Fatalf("Decode() error on data")
		}
		expectedFlows := []*schema.FlowMessage{
			{
				SamplingRate:    1,
				SrcAddr:         netip.MustParseAddr("fe80::d05b:45ff:feee:5ecf"),
				DstAddr:         netip.MustParseAddr("2001:db8::"),
				ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
				GotASPath:       false,
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:      72,
					schema.ColumnPackets:    1,
					schema.ColumnEType:      helpers.ETypeIPv6,
					schema.ColumnProto:      58,
					schema.ColumnSrcMAC:     0xd25b45ee5ecf,
					schema.ColumnDstMAC:     0xe2efc68f8cd4,
					schema.ColumnIPTTL:      255,
					schema.ColumnICMPv6Type: 135,
					// schema.ColumnICMPv6Code:   0,
				},
			},
		}
		for _, f := range got {
			f.TimeReceived = 0
		}

		if diff := helpers.Diff(got, expectedFlows); diff != "" {
			t.Fatalf("Decode() (-got, +want):\n%s", diff)
		}
	})
}
