// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

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
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()})

	// Send an option template
	template := helpers.ReadPcapL4(t, filepath.Join("testdata", "options-template.pcap"))
	got := nfdecoder.Decode(decoder.RawFlow{Payload: template, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on options template")
	}
	if len(got) != 0 {
		t.Fatalf("Decode() on options template got flows")
	}

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_inlet_flow_decoder_netflow_")
	expectedMetrics := map[string]string{
		`flows_total{exporter="127.0.0.1",version="9"}`:                                                                 "1",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send option data
	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "options-data.pcap"))
	got = nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on options data")
	}
	if len(got) != 0 {
		t.Fatalf("Decode() on options data got flows")
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_inlet_flow_decoder_netflow_")
	expectedMetrics = map[string]string{
		`flows_total{exporter="127.0.0.1",version="9"}`:                                                                 "2",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send a regular template
	template = helpers.ReadPcapL4(t, filepath.Join("testdata", "template.pcap"))
	got = nfdecoder.Decode(decoder.RawFlow{Payload: template, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on template")
	}
	if len(got) != 0 {
		t.Fatalf("Decode() on template got flows")
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_inlet_flow_decoder_netflow_")
	expectedMetrics = map[string]string{
		`flows_total{exporter="127.0.0.1",version="9"}`:                                                                 "3",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send data
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "data.pcap"))
	got = nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	if got == nil {
		t.Fatalf("Decode() error on data")
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
	gotMetrics = r.GetMetrics(
		"akvorado_inlet_flow_decoder_netflow_",
		"flows_total",
		"flowset_",
		"templates_",
	)
	expectedMetrics = map[string]string{
		`flows_total{exporter="127.0.0.1",version="9"}`:                                                                 "4",
		`flowset_records_sum{exporter="127.0.0.1",type="DataFlowSet",version="9"}`:                                      "4",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{exporter="127.0.0.1",type="DataFlowSet",version="9"}`:                                              "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestTemplatesMixedWithData(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t)})

	// Send packet with both data and templates
	template := helpers.ReadPcapL4(t, filepath.Join("testdata", "data+templates.pcap"))
	nfdecoder.Decode(decoder.RawFlow{Payload: template, Source: net.ParseIP("127.0.0.1")})

	// We don't really care about the data, but we should have accepted the
	// templates. Check the stats.
	gotMetrics := r.GetMetrics(
		"akvorado_inlet_flow_decoder_netflow_",
		"templates_",
	)
	expectedMetrics := map[string]string{
		`templates_total{exporter="127.0.0.1",obs_domain_id="17170432",template_id="256",type="options_template",version="9"}`: "1",
		`templates_total{exporter="127.0.0.1",obs_domain_id="17170432",template_id="257",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestDecodeSamplingRate(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()})

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "samplingrate-template.pcap"))
	got := nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "samplingrate-data.pcap"))
	got = append(got, nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})...)

	expectedFlows := []*schema.FlowMessage{
		{
			SamplingRate:    2048,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:232.131.215.65"),
			DstAddr:         netip.MustParseAddr("::ffff:142.183.180.65"),
			InIf:            13,
			SrcVlan:         701,
			NextHop:         netip.MustParseAddr("::ffff:0.0.0.0"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnPackets: 1,
				schema.ColumnBytes:   160,
				schema.ColumnProto:   6,
				schema.ColumnSrcPort: 13245,
				schema.ColumnDstPort: 10907,
				schema.ColumnEType:   helpers.ETypeIPv4,
			},
		},
	}

	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got[:1], expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}
}

func TestDecodeICMP(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()})

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "icmp-template.pcap"))
	got := nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "icmp-data.pcap"))
	got = append(got, nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})...)

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("2001:db8::"),
			DstAddr:         netip.MustParseAddr("2001:db8::1"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:   84,
				schema.ColumnEType:   2048,
				schema.ColumnPackets: 1,
				schema.ColumnProto:   1,
				// Type/Code  = 0
			},
		},
	}
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}

}

func TestDecodeDataLink(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()})

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "datalink-template.pcap"))
	got := nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})
	data = helpers.ReadPcapL4(t, filepath.Join("testdata", "datalink-data.pcap"))
	got = append(got, nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})...)

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("::ffff:51.51.51.51"),
			DstAddr:         netip.MustParseAddr("::ffff:52.52.52.52"),
			SrcVlan:         231,
			InIf:            582,
			OutIf:           0,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}

}

func TestDecodeMPLS(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t).EnableAllColumns()})

	data := helpers.ReadPcapL4(t, filepath.Join("testdata", "mpls.pcap"))
	got := nfdecoder.Decode(decoder.RawFlow{Payload: data, Source: net.ParseIP("127.0.0.1")})

	expectedFlows := []*schema.FlowMessage{
		{
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			SrcAddr:         netip.MustParseAddr("fd00::1:0:1:7:1"),
			DstAddr:         netip.MustParseAddr("fd00::1:0:1:5:1"),
			NextHop:         netip.MustParseAddr("::ffff:0.0.0.0"),
			SamplingRate:    1,
			OutIf:           16,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
			SamplingRate:    1,
			OutIf:           17,
			ProtobufDebug: map[schema.ColumnKey]interface{}{
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
	for _, f := range got {
		f.TimeReceived = 0
	}

	if diff := helpers.Diff(got, expectedFlows); diff != "" {
		t.Fatalf("Decode() (-got, +want):\n%s", diff)
	}

}
