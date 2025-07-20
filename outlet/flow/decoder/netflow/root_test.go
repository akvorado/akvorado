// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

func setup(t *testing.T, clearTs bool) (*reporter.Reporter, decoder.Decoder, *schema.FlowMessage, *[]*schema.FlowMessage, decoder.FinalizeFlowFunc) {
	t.Helper()
	r := reporter.NewMock(t)
	sch := schema.NewMock(t).EnableAllColumns()
	nfdecoder := New(r, decoder.Dependencies{Schema: sch})
	bf := sch.NewFlowMessage()
	got := []*schema.FlowMessage{}
	finalize := func() {
		if clearTs {
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
				schema.ColumnBytes:            1500,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv4,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          19624,
				schema.ColumnForwardingStatus: 64,
				schema.ColumnTCPFlags:         16,
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
				schema.ColumnBytes:            1500,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv4,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          2444,
				schema.ColumnForwardingStatus: 64,
				schema.ColumnTCPFlags:         16,
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
				schema.ColumnBytes:            1400,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv4,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          53697,
				schema.ColumnForwardingStatus: 64,
				schema.ColumnTCPFlags:         16,
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
				schema.ColumnBytes:            1448,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv4,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          52300,
				schema.ColumnForwardingStatus: 64,
				schema.ColumnTCPFlags:         16,
			},
		},
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
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
				schema.ColumnPackets: 1,
				schema.ColumnBytes:   160,
				schema.ColumnProto:   6,
				schema.ColumnSrcPort: 13245,
				schema.ColumnDstPort: 10907,
				schema.ColumnEType:   helpers.ETypeIPv4,
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
				schema.ColumnPackets:          18,
				schema.ColumnBytes:            1348,
				schema.ColumnProto:            6,
				schema.ColumnSrcPort:          443,
				schema.ColumnDstPort:          52616,
				schema.ColumnForwardingStatus: 64,
				schema.ColumnIPTTL:            127,
				schema.ColumnIPTos:            64,
				schema.ColumnIPv6FlowLabel:    252813,
				schema.ColumnTCPFlags:         16,
				schema.ColumnEType:            helpers.ETypeIPv6,
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
				schema.ColumnPackets:          4,
				schema.ColumnBytes:            579,
				schema.ColumnProto:            17,
				schema.ColumnSrcPort:          2121,
				schema.ColumnDstPort:          2121,
				schema.ColumnForwardingStatus: 64,
				schema.ColumnIPTTL:            57,
				schema.ColumnIPTos:            40,
				schema.ColumnIPv6FlowLabel:    570164,
				schema.ColumnEType:            helpers.ETypeIPv6,
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
				schema.ColumnBytes:      104,
				schema.ColumnDstPort:    32768,
				schema.ColumnEType:      34525,
				schema.ColumnICMPv6Type: 128, // Code: 0
				schema.ColumnPackets:    1,
				schema.ColumnProto:      58,
			},
		},
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("2001:db8::1"),
			DstAddr:         netip.MustParseAddr("2001:db8::"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:      104,
				schema.ColumnDstPort:    33024,
				schema.ColumnEType:      34525,
				schema.ColumnICMPv6Type: 129, // Code: 0
				schema.ColumnPackets:    1,
				schema.ColumnProto:      58,
			},
		},
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:203.0.113.4"),
			DstAddr:         netip.MustParseAddr("::ffff:203.0.113.5"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:      84,
				schema.ColumnDstPort:    2048,
				schema.ColumnEType:      2048,
				schema.ColumnICMPv4Type: 8, // Code: 0
				schema.ColumnPackets:    1,
				schema.ColumnProto:      1,
			},
		},
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:203.0.113.5"),
			DstAddr:         netip.MustParseAddr("::ffff:203.0.113.4"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:   84,
				schema.ColumnEType:   2048,
				schema.ColumnPackets: 1,
				schema.ColumnProto:   1,
				// Type/Code  = 0
			},
		},
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
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
				schema.ColumnBytes:        96,
				schema.ColumnSrcPort:      55501,
				schema.ColumnDstPort:      11777,
				schema.ColumnEType:        helpers.ETypeIPv4,
				schema.ColumnPackets:      1,
				schema.ColumnProto:        17,
				schema.ColumnSrcMAC:       0xb402165592f4,
				schema.ColumnDstMAC:       0x182ad36e503f,
				schema.ColumnIPFragmentID: 0x8f00,
				schema.ColumnIPTTL:        119,
			},
		},
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
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
	if diff := helpers.Diff(got, expectedFlows); diff != "" {
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
				schema.ColumnBytes:            89,
				schema.ColumnPackets:          1,
				schema.ColumnEType:            helpers.ETypeIPv6,
				schema.ColumnForwardingStatus: 66,
				schema.ColumnIPTTL:            255,
				schema.ColumnProto:            17,
				schema.ColumnSrcPort:          49153,
				schema.ColumnDstPort:          862,
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
				schema.ColumnBytes:            890,
				schema.ColumnPackets:          10,
				schema.ColumnEType:            helpers.ETypeIPv6,
				schema.ColumnForwardingStatus: 66,
				schema.ColumnIPTTL:            255,
				schema.ColumnProto:            17,
				schema.ColumnSrcPort:          49153,
				schema.ColumnDstPort:          862,
				schema.ColumnMPLSLabels:       []uint32{20006, 524275},
			},
		},
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeNFv5(t *testing.T) {
	for _, tsSource := range []pb.RawFlow_TimestampSource{
		pb.RawFlow_TS_NETFLOW_PACKET,
		pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED,
	} {
		t.Run(fmt.Sprintf("%s", tsSource), func(t *testing.T) {
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
						schema.ColumnBytes:    133,
						schema.ColumnPackets:  1,
						schema.ColumnEType:    helpers.ETypeIPv4,
						schema.ColumnProto:    6,
						schema.ColumnSrcPort:  30104,
						schema.ColumnDstPort:  11963,
						schema.ColumnTCPFlags: 0x18,
					},
				},
			}

			if diff := helpers.Diff((*got)[:1], expectedFlows); diff != "" {
				t.Errorf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestDecodeTimestampFromNetflowPacket(t *testing.T) {
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
	// all share the same timestamp with TimestampSourceNetflowPacket
	expectedTs := []uint32{
		1647285928,
		1647285928,
		1647285928,
		1647285928,
	}

	for i, flow := range *got {
		if flow.TimeReceived != expectedTs[i] {
			t.Errorf("Decode() (-got, +want):\n-%d, +%d", flow.TimeReceived, expectedTs[i])
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
	var packetTs uint32 = 1647285928
	expectedFirstSwitched := []uint32{
		944948659,
		944948659,
		944948660,
		944948661,
	}

	for i, flow := range *got {
		if val := packetTs - sysUptime + expectedFirstSwitched[i]; flow.TimeReceived != val {
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
				schema.ColumnSrcPort:    35303,
				schema.ColumnDstPort:    53,
				schema.ColumnSrcAddrNAT: netip.MustParseAddr("::ffff:10.143.52.29"),
				schema.ColumnDstAddrNAT: netip.MustParseAddr("::ffff:10.89.87.1"),
				schema.ColumnSrcPortNAT: 35303,
				schema.ColumnDstPortNAT: 53,
				schema.ColumnEType:      helpers.ETypeIPv4,
				schema.ColumnProto:      17,
			},
		},
	}

	if diff := helpers.Diff((*got)[:1], expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}
