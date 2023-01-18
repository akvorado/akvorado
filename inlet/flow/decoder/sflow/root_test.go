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
	sdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t)})

	// Send data
	data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-1140.pcap"))
	got := sdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on data")
	}
	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    1024,
			InIf:            27,
			OutIf:           28,
			SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
			DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:   1518,
				schema.ColumnPackets: 1,
				schema.ColumnEType:   helpers.ETypeIPv6,
				schema.ColumnProto:   6,
				schema.ColumnSrcPort: 46026,
				schema.ColumnDstPort: 22,
			},
		}, {
			SamplingRate:    1024,
			SrcAddr:         netip.MustParseAddr("::ffff:104.26.8.24"),
			DstAddr:         netip.MustParseAddr("::ffff:45.90.161.46"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			NextHop:         netip.MustParseAddr("::ffff:45.90.161.46"),
			InIf:            49001,
			OutIf:           25,
			SrcAS:           13335,
			DstAS:           39421,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:      439,
				schema.ColumnPackets:    1,
				schema.ColumnEType:      helpers.ETypeIPv4,
				schema.ColumnProto:      6,
				schema.ColumnSrcPort:    443,
				schema.ColumnDstPort:    56876,
				schema.ColumnSrcNetMask: 20,
				schema.ColumnDstNetMask: 27,
			},
		}, {
			SamplingRate:    1024,
			SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
			DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			InIf:            27,
			OutIf:           28,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:   1518,
				schema.ColumnPackets: 1,
				schema.ColumnEType:   helpers.ETypeIPv6,
				schema.ColumnProto:   6,
				schema.ColumnSrcPort: 46026,
				schema.ColumnDstPort: 22,
			},
		}, {
			SamplingRate:    1024,
			InIf:            28,
			OutIf:           49001,
			SrcAS:           39421,
			DstAS:           26615,
			SrcAddr:         netip.MustParseAddr("::ffff:45.90.161.148"),
			DstAddr:         netip.MustParseAddr("::ffff:191.87.91.27"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			NextHop:         netip.MustParseAddr("::ffff:31.14.69.110"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:      64,
				schema.ColumnPackets:    1,
				schema.ColumnEType:      helpers.ETypeIPv4,
				schema.ColumnProto:      6,
				schema.ColumnSrcPort:    55658,
				schema.ColumnDstPort:    5555,
				schema.ColumnSrcNetMask: 27,
				schema.ColumnDstNetMask: 17,
			},
		}, {
			SamplingRate:    1024,
			SrcAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:38"),
			DstAddr:         netip.MustParseAddr("2a0c:8880:2:0:185:21:130:39"),
			ExporterAddress: netip.MustParseAddr("::ffff:172.16.0.3"),
			InIf:            27,
			OutIf:           28,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:   1518,
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
	gotMetrics := r.GetMetrics(
		"akvorado_inlet_flow_decoder_sflow_",
		"count",
		"sample_",
	)
	expectedMetrics := map[string]string{
		`count{agent="172.16.0.3",exporter="127.0.0.1",version="5"}`:                                "1",
		`sample_records_sum{agent="172.16.0.3",exporter="127.0.0.1",type="FlowSample",version="5"}`: "14",
		`sample_sum{agent="172.16.0.3",exporter="127.0.0.1",type="FlowSample",version="5"}`:         "5",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestDecodeInterface(t *testing.T) {
	r := reporter.NewMock(t)
	sdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t)})

	t.Run("local interface", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-local-interface.pcap"))
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
					schema.ColumnBytes:   1518,
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
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-discard-interface.pcap"))
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
					schema.ColumnBytes:            1518,
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
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-multiple-interfaces.pcap"))
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
					schema.ColumnBytes:   1518,
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

	t.Run("expanded flow sample", func(t *testing.T) {
		// Send data
		data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-sflow-expanded-sample.pcap"))
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
				ProtobufDebug: map[schema.ColumnKey]interface{}{
					schema.ColumnBytes:      126,
					schema.ColumnPackets:    1,
					schema.ColumnEType:      helpers.ETypeIPv4,
					schema.ColumnProto:      6,
					schema.ColumnSrcPort:    22,
					schema.ColumnDstPort:    52237,
					schema.ColumnSrcNetMask: 32,
					schema.ColumnDstNetMask: 22,
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
