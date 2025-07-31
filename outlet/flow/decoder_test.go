// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"net"
	"net/netip"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
	"akvorado/outlet/flow/decoder/netflow"
	"akvorado/outlet/flow/decoder/sflow"
)

func TestFlowDecode(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	c, err := New(r, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	bf := sch.NewFlowMessage()
	got := []*schema.FlowMessage{}
	finalize := func() {
		bf.TimeReceived = 0
		// Keep a copy of the current flow message
		clone := *bf
		got = append(got, &clone)
		bf.Finalize()
	}

	// Get test data path
	_, src, _, _ := runtime.Caller(0)
	base := path.Join(path.Dir(src), "decoder", "netflow", "testdata")

	// Test NetFlow decoding
	t.Run("netflow", func(t *testing.T) {
		// Load template first
		templateData := helpers.ReadPcapL4(t, path.Join(base, "options-template.pcap"))
		templateRawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          templateData,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_NETFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}

		// Decode template (should return empty slice for templates)
		err := c.Decode(templateRawFlow, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() template error:\n%+v", err)
		}
		if len(got) != 0 {
			t.Logf("Template decode returned %d flows (expected 0)", len(got))
		}

		// Load options data
		optionsData := helpers.ReadPcapL4(t, path.Join(base, "options-data.pcap"))
		optionsRawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          optionsData,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_NETFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}

		// Decode options data
		err = c.Decode(optionsRawFlow, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() options data error:\n%+v", err)
		}
		if len(got) != 0 {
			t.Logf("Options data decode returned %d flows (expected 0)", len(got))
		}

		// Load template for actual data
		dataTemplateData := helpers.ReadPcapL4(t, path.Join(base, "template.pcap"))
		dataTemplateRawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          dataTemplateData,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_NETFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}

		// Decode data template
		err = c.Decode(dataTemplateRawFlow, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() data template error:\n%+v", err)
		}
		if len(got) != 0 {
			t.Logf("Data template decode returned %d flows (expected 0)", len(got))
		}

		// Load actual flow data
		flowData := helpers.ReadPcapL4(t, path.Join(base, "data.pcap"))
		flowRawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          flowData,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_NETFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}

		// Decode actual flow data
		err = c.Decode(flowRawFlow, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() flow data error:\n%+v", err)
		}
		if len(got) == 0 {
			t.Fatalf("Decode() returned no flows")
		}

		t.Logf("Successfully decoded %d flows", len(got))

		// Test with UseSourceAddress = true
		got = got[:0]
		flowRawFlow.UseSourceAddress = true
		err = c.Decode(flowRawFlow, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() with UseSourceAddress error:\n%+v", err)
		}
		if len(got) == 0 {
			t.Fatalf("Decode() with UseSourceAddress returned no flows")
		}

		// Verify exporter address was overridden (should be IPv4-mapped IPv6)
		expectedAddr := "::ffff:127.0.0.1"
		for _, flow := range got {
			if flow.ExporterAddress.String() != expectedAddr {
				t.Errorf("Expected exporter address %s, got %s", expectedAddr, flow.ExporterAddress.String())
			}
		}

		gotMetrics := r.GetMetrics("akvorado_outlet_flow_decoder_")
		expectedMetrics := map[string]string{
			`flows_total{name="netflow"}`:                                                                                                  "8",
			`netflow_packets_total{exporter="::ffff:127.0.0.1",version="9"}`:                                                               "5",
			`netflow_records_total{exporter="::ffff:127.0.0.1",type="DataFlowSet",version="9"}`:                                            "8",
			`netflow_records_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                     "4",
			`netflow_records_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                 "1",
			`netflow_records_total{exporter="::ffff:127.0.0.1",type="TemplateFlowSet",version="9"}`:                                        "1",
			`netflow_sets_total{exporter="::ffff:127.0.0.1",type="DataFlowSet",version="9"}`:                                               "2",
			`netflow_sets_total{exporter="::ffff:127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                        "1",
			`netflow_sets_total{exporter="::ffff:127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                    "1",
			`netflow_sets_total{exporter="::ffff:127.0.0.1",type="TemplateFlowSet",version="9"}`:                                           "1",
			`netflow_templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="257",type="options_template",version="9"}`: "1",
			`netflow_templates_total{exporter="::ffff:127.0.0.1",obs_domain_id="0",template_id="260",type="template",version="9"}`:         "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}
	})

	// Test sflow decoding
	t.Run("sflow", func(t *testing.T) {
		got = got[:0]
		sflowBase := path.Join(path.Dir(src), "decoder", "sflow", "testdata")
		flowData := helpers.ReadPcapL4(t, path.Join(sflowBase, "data-1140.pcap"))
		flowRawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          flowData,
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_SFLOW,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}

		err := c.Decode(flowRawFlow, bf, finalize)
		if err != nil {
			t.Fatalf("Decode() sflow error:\n%+v", err)
		}
		if len(got) == 0 {
			t.Fatalf("Decode() sflow returned no flows")
		}

		gotMetrics := r.GetMetrics("akvorado_outlet_flow_decoder_", "flows_total", "sflow_")
		expectedMetrics := map[string]string{
			`flows_total{name="netflow"}`: "8",
			`flows_total{name="sflow"}`:   "5",
			`sflow_flows_total{agent="172.16.0.3",exporter="::ffff:127.0.0.1",version="5"}`:                          "1",
			`sflow_sample_records_sum{agent="172.16.0.3",exporter="::ffff:127.0.0.1",type="FlowSample",version="5"}`: "14",
			`sflow_sample_sum{agent="172.16.0.3",exporter="::ffff:127.0.0.1",type="FlowSample",version="5"}`:         "5",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		t.Logf("Successfully decoded %d sflow flows", len(got))
	})

	// Test error cases
	t.Run("errors", func(t *testing.T) {
		// Unknown decoder
		got = got[:0]
		rawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().UnixNano()),
			Payload:          []byte("test"),
			SourceAddress:    net.ParseIP("127.0.0.1").To16(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_UNSPECIFIED,
			TimestampSource:  pb.RawFlow_TS_INPUT,
		}

		err := c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for unknown decoder")
		}

		// Missing source address
		rawFlow.Decoder = pb.RawFlow_DECODER_NETFLOW
		rawFlow.SourceAddress = nil
		err = c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for missing source address")
		}

		// Invalid payload for NetFlow
		rawFlow.Decoder = pb.RawFlow_DECODER_NETFLOW
		rawFlow.SourceAddress = net.ParseIP("127.0.0.1").To16()
		rawFlow.Payload = []byte("invalid")
		err = c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for invalid payload")
		}
		// Invalid payload for NetFlow v5
		rawFlow.Payload = []byte{0, 5, 11, 12, 13, 14}
		err = c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for invalid payload")
		}
		// Invalid payload for NetFlow v9
		rawFlow.Payload = []byte{0, 9, 11, 12, 13, 14}
		err = c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for invalid payload")
		}
		// Invalid payload for IPFIX
		rawFlow.Payload = []byte{0, 10, 11, 12, 13, 14}
		err = c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for invalid payload")
		}
		// Invalid payload for sFlow
		rawFlow.Decoder = pb.RawFlow_DECODER_SFLOW
		err = c.Decode(rawFlow, bf, finalize)
		if err == nil {
			t.Fatal("Expected error for invalid payload")
		}

		gotMetrics := r.GetMetrics("akvorado_outlet_flow_decoder_", "errors", "netflow_errors", "sflow_errors")
		expectedMetrics := map[string]string{
			`errors_total{name="netflow"}`: "4",
			`errors_total{name="sflow"}`:   "1",
			`netflow_errors_total{error="IPFIX decoding error",exporter="::ffff:127.0.0.1"}`:      "1",
			`netflow_errors_total{error="NetFlow v5 decoding error",exporter="::ffff:127.0.0.1"}`: "1",
			`netflow_errors_total{error="NetFlow v9 decoding error",exporter="::ffff:127.0.0.1"}`: "1",
			`sflow_errors_total{error="sFlow decoding error",exporter="::ffff:127.0.0.1"}`:        "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

	})
}

func BenchmarkDecodeNetFlow(b *testing.B) {
	schema.DisableDebug(b)
	r := reporter.NewMock(b)
	sch := schema.NewMock(b)
	bf := sch.NewFlowMessage()
	finalize := func() {}
	nfdecoder := netflow.New(r, decoder.Dependencies{Schema: sch})
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}

	template := helpers.ReadPcapL4(b, filepath.Join("decoder", "netflow", "testdata", "options-template.pcap"))
	_, err := nfdecoder.Decode(
		decoder.RawFlow{Payload: template, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		b.Fatalf("Decode() error on options template:\n%+v", err)
	}
	data := helpers.ReadPcapL4(b, filepath.Join("decoder", "netflow", "testdata", "options-data.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		b.Fatalf("Decode() error on options data:\n%+v", err)
	}
	template = helpers.ReadPcapL4(b, filepath.Join("decoder", "netflow", "testdata", "template.pcap"))
	_, err = nfdecoder.Decode(
		decoder.RawFlow{Payload: template, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
		options, bf, finalize)
	if err != nil {
		b.Fatalf("Decode() error on template:\n%+v", err)
	}
	data = helpers.ReadPcapL4(b, filepath.Join("decoder", "netflow", "testdata", "data.pcap"))

	for b.Loop() {
		nfdecoder.Decode(
			decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
			options, bf, finalize)
	}
}

func BenchmarkDecodeSflow(b *testing.B) {
	schema.DisableDebug(b)
	r := reporter.NewMock(b)
	sch := schema.NewMock(b)
	bf := sch.NewFlowMessage()
	finalize := func() {}
	sdecoder := sflow.New(r, decoder.Dependencies{Schema: sch})
	options := decoder.Option{TimestampSource: pb.RawFlow_TS_INPUT}
	data := helpers.ReadPcapL4(b, filepath.Join("decoder", "sflow", "testdata", "data-1140.pcap"))

	for b.Loop() {
		sdecoder.Decode(
			decoder.RawFlow{Payload: data, Source: netip.MustParseAddr("::ffff:127.0.0.1")},
			options, bf, finalize)
	}
}
