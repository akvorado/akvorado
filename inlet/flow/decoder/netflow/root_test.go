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
	template := helpers.ReadPcapPayload(t, filepath.Join("testdata", "options-template-257.pcap"))
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
		`count{exporter="127.0.0.1",version="9"}`:                                                                       "1",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send option data
	data := helpers.ReadPcapPayload(t, filepath.Join("testdata", "options-data-257.pcap"))
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
		`count{exporter="127.0.0.1",version="9"}`:                                                                       "2",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send a regular template
	template = helpers.ReadPcapPayload(t, filepath.Join("testdata", "template-260.pcap"))
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
		`count{exporter="127.0.0.1",version="9"}`:                                                                       "3",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send data
	data = helpers.ReadPcapPayload(t, filepath.Join("testdata", "data-260.pcap"))
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
		"count",
		"flowset_",
		"templates_",
	)
	expectedMetrics = map[string]string{
		`count{exporter="127.0.0.1",version="9"}`:                                                                       "4",
		`flowset_records_sum{exporter="127.0.0.1",type="DataFlowSet",version="9"}`:                                      "4",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{exporter="127.0.0.1",type="DataFlowSet",version="9"}`:                                              "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{exporter="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{exporter="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestTemplatesMixedWithData(t *testing.T) {
	r := reporter.NewMock(t)
	nfdecoder := New(r, decoder.Dependencies{Schema: schema.NewMock(t)})

	// Send packet with both data and templates
	template := helpers.ReadPcapPayload(t, filepath.Join("testdata", "data+templates-256-257.pcap"))
	nfdecoder.Decode(decoder.RawFlow{Payload: template, Source: net.ParseIP("127.0.0.1")})

	// We don't really care about the data, but we should have accepted the
	// templates. Check the stats.
	gotMetrics := r.GetMetrics(
		"akvorado_inlet_flow_decoder_netflow_",
		"templates_",
	)
	expectedMetrics := map[string]string{
		`templates_count{exporter="127.0.0.1",obs_domain_id="17170432",template_id="256",type="options_template",version="9"}`: "1",
		`templates_count{exporter="127.0.0.1",obs_domain_id="17170432",template_id="257",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}
