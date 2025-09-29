// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"

	"github.com/google/go-cmp/cmp/cmpopts"
)

func setup(t *testing.T, clearTS bool) (*reporter.Reporter, decoder.Decoder, *schema.FlowMessage, *[]*schema.FlowMessage, decoder.FinalizeFlowFunc) {
	t.Helper()
	r := reporter.NewMock(t)
	sch := schema.NewMock(t).EnableAllColumns()
	nfdecoder := New(r, decoder.Dependencies{Schema: sch})
	bf := sch.NewFlowMessage()
	got := []*schema.FlowMessage{}
	finalize := func() {
		if clearTS {
			bf.TimeReceived = 0
		}
		// Keep a copy of the current flow message
		clone := *bf
		got = append(got, &clone)
		// And clear the flow message
		bf.Clear()
	}
	return r, nfdecoder, bf, &got, finalize
}

func TestDecode(t *testing.T) {
	r, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	// Send an option template
	template := helpers.ReadPcapL4(t, filepath.Join("testdata", "options-template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: template, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error on options template:\n%+v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("Decode() on options template got flows:\n%+v", *got)
	}

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_outlet_flow_decoder_netflow_")
	expectedMetrics := map[string]string{
		`packets_total{exporter="::ffff:127.0.0.1",version="9"}`:                                                               "1",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                 "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                    "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send option data
	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "options-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error on options data:\n%+v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("Decode() on options data got flows")
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_outlet_flow_decoder_netflow_")
	expectedMetrics = map[string]string{
		`packets_total{exporter="::ffff:127.0.0.1",version="9"}`:                                                               "2",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                 "1",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                     "4",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                    "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                        "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send a regular template
	template = helpers.ReadPcapL4(t, filepath.Join("testdata", "template.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: template, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error on template:\n%+v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("Decode() on template got flows")
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_outlet_flow_decoder_netflow_")
	expectedMetrics = map[string]string{
		`packets_total{exporter="::ffff:127.0.0.1",version="9"}`:                                                               "3",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                 "1",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                     "4",
		`records_total{exporter="::ffff:127.0.0.1",type="TemplateFlowSet",version="9"}`:                                        "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                    "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                        "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="TemplateFlowSet",version="9"}`:                                           "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send data
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error on data:\n%+v", err)
	}
	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:198.38.121.178"),
			DstAddr:         netip.MustParseAddr("::ffff:91.170.143.87"),
			NextHop:         netip.MustParseAddr("::ffff:194.149.174.63"),
			InIf:            335,
			OutIf:           450,
			SrcNetMask:      24,
			DstNetMask:      14,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            uint64(1500),
				schema.ColumnPackets:          uint64(1),
				schema.ColumnEType:            uint32(helpers.ETypeIPv4),
				schema.ColumnProto:            uint32(6),
				schema.ColumnSrcPort:          uint16(443),
				schema.ColumnDstPort:          uint16(19624),
				schema.ColumnForwardingStatus: uint32(64),
				schema.ColumnTCPFlags:         uint16(16),
			},
		}, {
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:198.38.121.219"),
			DstAddr:         netip.MustParseAddr("::ffff:88.122.57.97"),
			InIf:            335,
			OutIf:           452,
			NextHop:         netip.MustParseAddr("::ffff:194.149.174.71"),
			SrcNetMask:      24,
			DstNetMask:      14,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            uint64(1500),
				schema.ColumnPackets:          uint64(1),
				schema.ColumnEType:            uint32(helpers.ETypeIPv4),
				schema.ColumnProto:            uint32(6),
				schema.ColumnSrcPort:          uint16(443),
				schema.ColumnDstPort:          uint16(2444),
				schema.ColumnForwardingStatus: uint32(64),
				schema.ColumnTCPFlags:         uint16(16),
			},
		}, {
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:173.194.190.106"),
			DstAddr:         netip.MustParseAddr("::ffff:37.165.129.20"),
			InIf:            461,
			OutIf:           306,
			NextHop:         netip.MustParseAddr("::ffff:252.223.0.0"),
			SrcNetMask:      20,
			DstNetMask:      18,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            uint64(1400),
				schema.ColumnPackets:          uint64(1),
				schema.ColumnEType:            uint32(helpers.ETypeIPv4),
				schema.ColumnProto:            uint32(6),
				schema.ColumnSrcPort:          uint16(443),
				schema.ColumnDstPort:          uint16(53697),
				schema.ColumnForwardingStatus: uint32(64),
				schema.ColumnTCPFlags:         uint16(16),
			},
		}, {
			SamplingRate:    30000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:74.125.100.234"),
			DstAddr:         netip.MustParseAddr("::ffff:88.120.219.117"),
			NextHop:         netip.MustParseAddr("::ffff:194.149.174.61"),
			InIf:            461,
			OutIf:           451,
			SrcNetMask:      16,
			DstNetMask:      14,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            uint64(1448),
				schema.ColumnPackets:          uint64(1),
				schema.ColumnEType:            uint32(helpers.ETypeIPv4),
				schema.ColumnProto:            uint32(6),
				schema.ColumnSrcPort:          uint16(443),
				schema.ColumnDstPort:          uint16(52300),
				schema.ColumnForwardingStatus: uint32(64),
				schema.ColumnTCPFlags:         uint16(16),
			},
		},
	}

	if diff := helpers.Diff(got, &expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
	gotMetrics = r.GetMetrics(
		"akvorado_outlet_flow_decoder_netflow_",
		"packets_",
		"sets_",
		"records_",
		"templates_",
	)
	expectedMetrics = map[string]string{
		`packets_total{exporter="::ffff:127.0.0.1",version="9"}`:                                                               "4",
		`records_total{exporter="::ffff:127.0.0.1",type="DataFlowSet",version="9"}`:                                            "4",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                     "4",
		`records_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                 "1",
		`records_total{exporter="::ffff:127.0.0.1",type="TemplateFlowSet",version="9"}`:                                        "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="DataFlowSet",version="9"}`:                                               "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                        "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                    "1",
		`sets_total{exporter="::ffff:127.0.0.1",type="TemplateFlowSet",version="9"}`:                                           "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestTemplatesMixedWithData(t *testing.T) {
	r, nfdecoder, bf, _, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	// Send packet with both data and templates
	template := helpers.ReadPcapL4(t, filepath.Join("testdata", "data+templates.pcap"))
	nfdecoder.Decode(
		decoder.RawFlow{Payload: template, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)

	// We don't really care about the data, but we should have accepted the
	// templates. Check the stats.
	gotMetrics := r.GetMetrics(
		"akvorado_outlet_flow_decoder_netflow_",
		"templates_",
	)
	expectedMetrics := map[string]string{
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="17170432",template_id="256",type="options_template",version="9"}`: "1",
		`templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="17170432",template_id="257",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestDecodeSamplingRate(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "samplingrate-template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "samplingrate-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    2048,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:232.131.215.65"),
			DstAddr:         netip.MustParseAddr("::ffff:142.183.180.65"),
			InIf:            13,
			SrcVlan:         701,
			NextHop:         netip.MustParseAddr("::ffff:0.0.0.0"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnPackets: uint64(1),
				schema.ColumnBytes:   uint64(160),
				schema.ColumnProto:   uint32(6),
				schema.ColumnSrcPort: uint16(13245),
				schema.ColumnDstPort: uint16(10907),
				schema.ColumnEType:   uint32(helpers.ETypeIPv4),
			},
		},
	}

	if diff := helpers.Diff((*got)[:1], expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeMultipleSamplingRates(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "multiplesamplingrates-options-template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "multiplesamplingrates-options-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "multiplesamplingrates-template.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "multiplesamplingrates-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    4000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("ffff::68"),
			DstAddr:         netip.MustParseAddr("ffff::1a"),
			NextHop:         netip.MustParseAddr("ffff::2"),
			SrcNetMask:      48,
			DstNetMask:      56,
			InIf:            97,
			OutIf:           6,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnPackets:          uint64(18),
				schema.ColumnBytes:            uint64(1348),
				schema.ColumnProto:            uint32(6),
				schema.ColumnSrcPort:          uint16(443),
				schema.ColumnDstPort:          uint16(52616),
				schema.ColumnForwardingStatus: uint32(64),
				schema.ColumnIPTTL:            uint8(127),
				schema.ColumnIPTos:            uint8(64),
				schema.ColumnIPv6FlowLabel:    uint32(252813),
				schema.ColumnTCPFlags:         uint16(16),
				schema.ColumnEType:            uint32(helpers.ETypeIPv6),
			},
		},
		{
			SamplingRate:    2000,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("ffff::5a"),
			DstAddr:         netip.MustParseAddr("ffff::f"),
			NextHop:         netip.MustParseAddr("ffff::3c"),
			SrcNetMask:      36,
			DstNetMask:      48,
			InIf:            103,
			OutIf:           6,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnPackets:          uint64(4),
				schema.ColumnBytes:            uint64(579),
				schema.ColumnProto:            uint32(17),
				schema.ColumnSrcPort:          uint16(2121),
				schema.ColumnDstPort:          uint16(2121),
				schema.ColumnForwardingStatus: uint32(64),
				schema.ColumnIPTTL:            uint8(57),
				schema.ColumnIPTos:            uint8(40),
				schema.ColumnIPv6FlowLabel:    uint32(570164),
				schema.ColumnEType:            uint32(helpers.ETypeIPv6),
			},
		},
	}

	if diff := helpers.Diff((*got)[:2], expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeICMP(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "icmp-template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "icmp-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("2001:db8::"),
			DstAddr:         netip.MustParseAddr("2001:db8::1"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:      uint64(104),
				schema.ColumnDstPort:    uint16(32768),
				schema.ColumnEType:      uint32(34525),
				schema.ColumnICMPv6Type: uint8(128), // Code: 0
				schema.ColumnPackets:    uint64(1),
				schema.ColumnProto:      uint32(58),
			},
		},
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("2001:db8::1"),
			DstAddr:         netip.MustParseAddr("2001:db8::"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:      uint64(104),
				schema.ColumnDstPort:    uint16(33024),
				schema.ColumnEType:      uint32(34525),
				schema.ColumnICMPv6Type: uint8(129), // Code: 0
				schema.ColumnPackets:    uint64(1),
				schema.ColumnProto:      uint32(58),
			},
		},
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:203.0.113.4"),
			DstAddr:         netip.MustParseAddr("::ffff:203.0.113.5"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:      uint64(84),
				schema.ColumnDstPort:    uint16(2048),
				schema.ColumnEType:      uint32(2048),
				schema.ColumnICMPv4Type: uint8(8), // Code: 0
				schema.ColumnPackets:    uint64(1),
				schema.ColumnProto:      uint32(1),
			},
		},
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:203.0.113.5"),
			DstAddr:         netip.MustParseAddr("::ffff:203.0.113.4"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:   uint64(84),
				schema.ColumnEType:   uint32(2048),
				schema.ColumnPackets: uint64(1),
				schema.ColumnProto:   uint32(1),
				// Type/Code  = 0
			},
		},
	}

	if diff := helpers.Diff(got, &expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeDataLink(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "datalink-template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "datalink-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:51.51.51.51"),
			DstAddr:         netip.MustParseAddr("::ffff:52.52.52.52"),
			SrcVlan:         231,
			InIf:            582,
			OutIf:           0,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:        uint64(96),
				schema.ColumnSrcPort:      uint16(55501),
				schema.ColumnDstPort:      uint16(11777),
				schema.ColumnEType:        uint32(helpers.ETypeIPv4),
				schema.ColumnPackets:      uint64(1),
				schema.ColumnProto:        uint32(17),
				schema.ColumnSrcMAC:       uint64(0xb402165592f4),
				schema.ColumnDstMAC:       uint64(0x182ad36e503f),
				schema.ColumnIPFragmentID: uint32(0x8f00),
				schema.ColumnIPTTL:        uint8(119),
			},
		},
	}

	if diff := helpers.Diff(got, &expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeWithoutTemplate(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "datalink-data.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{}
	if diff := helpers.Diff(got, &expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeMPLS(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "mpls.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("fd00::1:0:1:7:1"),
			DstAddr:         netip.MustParseAddr("fd00::1:0:1:5:1"),
			NextHop:         netip.MustParseAddr("::ffff:0.0.0.0"),
			SamplingRate:    10,
			OutIf:           16,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            uint64(89),
				schema.ColumnPackets:          uint64(1),
				schema.ColumnEType:            uint32(helpers.ETypeIPv6),
				schema.ColumnForwardingStatus: uint32(66),
				schema.ColumnIPTTL:            uint8(255),
				schema.ColumnProto:            uint32(17),
				schema.ColumnSrcPort:          uint16(49153),
				schema.ColumnDstPort:          uint16(862),
				schema.ColumnMPLSLabels:       []uint32{20005, 524250},
			},
		}, {
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("fd00::1:0:1:7:1"),
			DstAddr:         netip.MustParseAddr("fd00::1:0:1:6:1"),
			NextHop:         netip.MustParseAddr("::ffff:0.0.0.0"),
			SamplingRate:    10,
			OutIf:           17,
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:            uint64(890),
				schema.ColumnPackets:          uint64(10),
				schema.ColumnEType:            uint32(helpers.ETypeIPv6),
				schema.ColumnForwardingStatus: uint32(66),
				schema.ColumnIPTTL:            uint8(255),
				schema.ColumnProto:            uint32(17),
				schema.ColumnSrcPort:          uint16(49153),
				schema.ColumnDstPort:          uint16(862),
				schema.ColumnMPLSLabels:       []uint32{20006, 524275},
			},
		},
	}

	if diff := helpers.Diff(got, &expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeNFv5(t *testing.T) {
	for _, tsSource := range []pb.RawFlow_TimestampSource{
		pb.RawFlow_TS_NETFLOW_PACKET,
		pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED,
	} {
		t.Run(tsSource.String(), func(t *testing.T) {
			_, nfdecoder, bf, got, finalize := setup(t, false)
			options := decoder.Option{TimestampSource: tsSource}

			data := helpers.ReadPcapL4(t, filepath.Join("testdata", "nfv5.pcap"))
			_, err := nfdecoder.Decode(
				decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
				options, bf, finalize)
			if err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}

			ts := uint32(1680626679)
			if tsSource == pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED {
				ts = 1680611679
			}

			expectedFlows := []*schema.FlowMessage{
				{
					TimeReceived:    ts,
					ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
					SrcAddr:         netip.MustParseAddr("::ffff:161.202.212.212"),
					DstAddr:         netip.MustParseAddr("::ffff:202.152.70.24"),
					NextHop:         netip.MustParseAddr("::ffff:61.6.255.150"),
					SamplingRate:    1,
					InIf:            117,
					OutIf:           86,
					SrcAS:           36351,
					DstAS:           10101,
					SrcNetMask:      19,
					DstNetMask:      24,
					OtherColumns: map[schema.ColumnKey]any{
						schema.ColumnBytes:    uint64(133),
						schema.ColumnPackets:  uint64(1),
						schema.ColumnEType:    uint32(helpers.ETypeIPv4),
						schema.ColumnProto:    uint32(6),
						schema.ColumnSrcPort:  uint16(30104),
						schema.ColumnDstPort:  uint16(11963),
						schema.ColumnTCPFlags: uint16(0x18),
					},
				},
			}

			if diff := helpers.Diff((*got)[:1], expectedFlows); diff != "" {
				t.Errorf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestDecodeTimestampFromNetFlowPacket(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, false)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_NETFLOW_PACKET}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	// 4 flows in capture
	// all share the same timestamp with TimestampSourceNetFlowPacket
	expectedTS := []uint32{
		1647285928,
		1647285928,
		1647285928,
		1647285928,
	}

	for i, flow := range *got {
		if flow.TimeReceived != expectedTS[i] {
			t.Errorf("Decode() (-got, +want):\n-%d, +%d", flow.TimeReceived, expectedTS[i])
		}
	}
}

func TestDecodeTimestampFromFirstSwitched(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, false)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	// 4 flows in capture
	var sysUptime uint32 = 944951609
	var packetTS uint32 = 1647285928
	expectedFirstSwitched := []uint32{
		944948659,
		944948659,
		944948660,
		944948661,
	}

	for i, flow := range *got {
		if val := packetTS - sysUptime + expectedFirstSwitched[i]; flow.TimeReceived != val {
			t.Errorf("Decode() (-got, +want):\n-%d, +%d", flow.TimeReceived, val)
		}
	}
}

func TestDecodeNAT(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	// The following PCAP is a NAT event, there is no sampling rate, no bytes,
	// no packets. We can't do much with it.
	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "nat.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:172.16.100.198"),
			DstAddr:         netip.MustParseAddr("::ffff:10.89.87.1"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcPort:    uint16(35303),
				schema.ColumnDstPort:    uint16(53),
				schema.ColumnSrcAddrNAT: netip.MustParseAddr("::ffff:10.143.52.29"),
				schema.ColumnDstAddrNAT: netip.MustParseAddr("::ffff:10.89.87.1"),
				schema.ColumnSrcPortNAT: uint16(35303),
				schema.ColumnDstPortNAT: uint16(53),
				schema.ColumnEType:      uint32(helpers.ETypeIPv4),
				schema.ColumnProto:      uint32(17),
			},
		},
	}

	if diff := helpers.Diff((*got)[:1], expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodePhysicalInterfaces(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "physicalinterfaces.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    1000,
			InIf:            1342177291,
			OutIf:           0,
			SrcVlan:         4,
			DstVlan:         0,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:147.53.240.75"),
			DstAddr:         netip.MustParseAddr("::ffff:212.82.101.24"),
			NextHop:         netip.MustParseAddr("::"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcMAC:   uint64(0xc014fef6c365),
				schema.ColumnDstMAC:   uint64(0xe8b6c24ae34c),
				schema.ColumnPackets:  uint64(3),
				schema.ColumnBytes:    uint64(4506),
				schema.ColumnSrcPort:  uint16(55629),
				schema.ColumnDstPort:  uint16(993),
				schema.ColumnTCPFlags: uint16(0x10),
				schema.ColumnEType:    uint32(helpers.ETypeIPv4),
				schema.ColumnProto:    uint32(6),
			},
		},
	}

	if diff := helpers.Diff((*got)[:1], expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeRFC5103(t *testing.T) {
	_, nfdecoder, bf, got, finalize := setup(t, true)
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "ipfixprobe-templates.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "ipfixprobe-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		t.Fatalf("Decode() error:\n%+v", err)
	}

	expectedFlows := []*schema.FlowMessage{
		{
			// First biflow, direct
			SamplingRate:    0,
			InIf:            10,
			OutIf:           0,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:10.10.1.4"),
			DstAddr:         netip.MustParseAddr("::ffff:10.10.1.1"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcMAC:  uint64(0x00e01c3c17c2),
				schema.ColumnDstMAC:  uint64(0x001f33d98160),
				schema.ColumnPackets: uint64(1),
				schema.ColumnBytes:   uint64(62),
				schema.ColumnSrcPort: uint16(56166),
				schema.ColumnDstPort: uint16(53),
				schema.ColumnEType:   uint32(helpers.ETypeIPv4),
				schema.ColumnProto:   uint32(17),
			},
		}, {
			// First biflow, reverse
			SamplingRate:    0,
			InIf:            0,
			OutIf:           10,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:10.10.1.1"),
			DstAddr:         netip.MustParseAddr("::ffff:10.10.1.4"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnDstMAC:  uint64(0x00e01c3c17c2),
				schema.ColumnSrcMAC:  uint64(0x001f33d98160),
				schema.ColumnPackets: uint64(1),
				schema.ColumnBytes:   uint64(128),
				schema.ColumnDstPort: uint16(56166),
				schema.ColumnSrcPort: uint16(53),
				schema.ColumnEType:   uint32(helpers.ETypeIPv4),
				schema.ColumnProto:   uint32(17),
			},
		}, {
			// Second biflow, direct, no reverse
			SamplingRate:    0,
			InIf:            10,
			OutIf:           0,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:10.10.1.20"),
			DstAddr:         netip.MustParseAddr("::ffff:10.10.1.255"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcMAC:  uint64(0x00023fec6111),
				schema.ColumnDstMAC:  uint64(0xffffffffffff),
				schema.ColumnPackets: uint64(1),
				schema.ColumnBytes:   uint64(229),
				schema.ColumnSrcPort: uint16(138),
				schema.ColumnDstPort: uint16(138),
				schema.ColumnEType:   uint32(helpers.ETypeIPv4),
				schema.ColumnProto:   uint32(17),
			},
		}, {
			// Third biflow, direct
			SamplingRate:    0,
			InIf:            10,
			OutIf:           0,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:10.10.1.4"),
			DstAddr:         netip.MustParseAddr("::ffff:74.53.140.153"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcMAC:   uint64(0x00e01c3c17c2),
				schema.ColumnDstMAC:   uint64(0x001f33d98160),
				schema.ColumnPackets:  uint64(28),
				schema.ColumnBytes:    uint64(21673),
				schema.ColumnSrcPort:  uint16(1470),
				schema.ColumnDstPort:  uint16(25),
				schema.ColumnEType:    uint32(helpers.ETypeIPv4),
				schema.ColumnProto:    uint32(6),
				schema.ColumnTCPFlags: uint16(0x1b),
			},
		}, {
			// Third biflow, reverse
			SamplingRate:    0,
			InIf:            0,
			OutIf:           10,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:74.53.140.153"),
			DstAddr:         netip.MustParseAddr("::ffff:10.10.1.4"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcMAC:   uint64(0x001f33d98160),
				schema.ColumnDstMAC:   uint64(0x00e01c3c17c2),
				schema.ColumnPackets:  uint64(25),
				schema.ColumnBytes:    uint64(1546),
				schema.ColumnSrcPort:  uint16(25),
				schema.ColumnDstPort:  uint16(1470),
				schema.ColumnEType:    uint32(helpers.ETypeIPv4),
				schema.ColumnProto:    uint32(6),
				schema.ColumnTCPFlags: uint16(0x1b),
			},
		}, {
			// Last biflow, direct, no reverse
			SamplingRate:    0,
			InIf:            10,
			OutIf:           0,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:192.168.1.1"),
			DstAddr:         netip.MustParseAddr("::ffff:10.10.1.4"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnSrcMAC:  uint64(0x001f33d98160),
				schema.ColumnDstMAC:  uint64(0x00e01c3c17c2),
				schema.ColumnPackets: uint64(4),
				schema.ColumnBytes:   uint64(2304),
				schema.ColumnEType:   uint32(helpers.ETypeIPv4),
				schema.ColumnProto:   uint32(1),
			},
		},
	}

	if diff := helpers.Diff((*got), expectedFlows, cmpopts.SortSlices(func(a, b *schema.FlowMessage) int {
		return strings.Compare(fmt.Sprintf("%+v", a), fmt.Sprintf("%+v", b))
	})); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}

}
