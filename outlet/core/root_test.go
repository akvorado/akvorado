// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"akvorado/common/constants"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/flow"
	"akvorado/outlet/kafka"
	"akvorado/outlet/metadata"
	"akvorado/outlet/routing"
)

func TestCore(t *testing.T) {
	r := reporter.NewMock(t)

	// Prepare all components.
	daemonComponent := daemon.NewMock(t)
	metadataComponent := metadata.NewMock(t, r, metadata.DefaultConfiguration(),
		metadata.Dependencies{Daemon: daemonComponent})
	flowComponent, err := flow.New(r, flow.DefaultConfiguration(), flow.Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("flow.New() error:\n%+v", err)
	}
	httpComponent := httpserver.NewMock(t, r)
	routingComponent := routing.NewMock(t, r)
	routingComponent.PopulateRIB(t)
	kafkaComponent, incoming := kafka.NewMock(t, kafka.DefaultConfiguration())
	var clickhouseMessages []*schema.FlowMessage
	var clickhouseMessagesMutex sync.Mutex
	clickhouseComponent := clickhouse.NewMock(t, func(msg *schema.FlowMessage) {
		clickhouseMessagesMutex.Lock()
		defer clickhouseMessagesMutex.Unlock()
		clickhouseMessages = append(clickhouseMessages, msg)
	})

	// Instantiate and start core
	sch := schema.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon:     daemonComponent,
		Flow:       flowComponent,
		Metadata:   metadataComponent,
		Kafka:      kafkaComponent,
		ClickHouse: clickhouseComponent,
		HTTP:       httpComponent,
		Routing:    routingComponent,
		Schema:     sch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	flowMessage := func(exporter string, in, out uint32) *schema.FlowMessage {
		msg := &schema.FlowMessage{
			TimeReceived:    200,
			SamplingRate:    1000,
			ExporterAddress: helpers.AddrTo6(netip.MustParseAddr(exporter)),
			InIf:            in,
			OutIf:           out,
			SrcAddr:         netip.MustParseAddr("::ffff:67.43.156.77"),
			DstAddr:         netip.MustParseAddr("::ffff:2.125.160.216"),
			OtherColumns: map[schema.ColumnKey]any{
				schema.ColumnBytes:   uint64(6765),
				schema.ColumnPackets: uint64(4),
				schema.ColumnEType:   uint32(0x800),
				schema.ColumnProto:   uint32(constants.ProtoTCP),
				schema.ColumnSrcPort: uint16(8534),
				schema.ColumnDstPort: uint16(80),
			},
		}
		return msg
	}

	expectedFlowMessage := func(exporter string, in, out uint32) *schema.FlowMessage {
		expected := flowMessage(exporter, in, out)
		expected.SrcAS = 0 // no geoip enrich anymore
		expected.DstAS = 0 // no geoip enrich anymore
		expected.OtherColumns[schema.ColumnInIfName] = fmt.Sprintf("Gi0/0/%d", in)
		expected.OtherColumns[schema.ColumnOutIfName] = fmt.Sprintf("Gi0/0/%d", out)
		expected.OtherColumns[schema.ColumnInIfDescription] = fmt.Sprintf("Interface %d", in)
		expected.OtherColumns[schema.ColumnOutIfDescription] = fmt.Sprintf("Interface %d", out)
		expected.OtherColumns[schema.ColumnInIfSpeed] = uint32(1000)
		expected.OtherColumns[schema.ColumnOutIfSpeed] = uint32(1000)
		expected.OtherColumns[schema.ColumnExporterName] = strings.ReplaceAll(exporter, ".", "_")
		return expected
	}

	// Helper function to inject flows using the new mechanism
	injectFlow := func(flow *schema.FlowMessage, rateLimit uint64) {
		t.Helper()
		var buf bytes.Buffer
		encoder := gob.NewEncoder(&buf)
		if err := encoder.Encode(flow); err != nil {
			t.Fatalf("gob.Encode() error: %v", err)
		}

		rawFlow := &pb.RawFlow{
			TimeReceived:     uint64(time.Now().Unix()),
			Payload:          buf.Bytes(),
			SourceAddress:    flow.ExporterAddress.AsSlice(),
			UseSourceAddress: false,
			Decoder:          pb.RawFlow_DECODER_GOB,
			TimestampSource:  pb.RawFlow_TS_INPUT,
			RateLimit:        rateLimit,
		}

		data, err := proto.Marshal(rawFlow)
		if err != nil {
			t.Fatalf("proto.Marshal() error: %v", err)
		}

		// Send to kafka mock's incoming channel
		incoming <- data
	}

	t.Run("core", func(t *testing.T) {
		clickhouseMessagesMutex.Lock()
		clickhouseMessages = clickhouseMessages[:0]
		clickhouseMessagesMutex.Unlock()

		// Inject several messages
		injectFlow(flowMessage("192.0.2.142", 434, 677), 0)
		injectFlow(flowMessage("192.0.2.143", 437, 679), 0)
		time.Sleep(20 * time.Millisecond)

		gotMetrics := r.GetMetrics("akvorado_outlet_core_", "-flows_processing_")
		expectedMetrics := map[string]string{
			`classifier_exporter_cache_items_total`:         "0",
			`classifier_interface_cache_items_total`:        "0",
			`received_flows_total{exporter="192.0.2.142"}`:  "1",
			`received_flows_total{exporter="192.0.2.143"}`:  "1",
			`forwarded_flows_total{exporter="192.0.2.142"}`: "1",
			`forwarded_flows_total{exporter="192.0.2.143"}`: "1",
			`received_raw_flows_total`:                      "2",
			`flows_http_clients`:                            "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Should have 2 more flows in clickhouseMessages now
		clickhouseMessagesMutex.Lock()
		clickhouseMessagesLen := len(clickhouseMessages)
		clickhouseMessagesMutex.Unlock()
		if clickhouseMessagesLen < 2 {
			t.Fatalf("Expected at least 2 flows in clickhouseMessages, got %d", clickhouseMessagesLen)
		}

		// Now, check we get the message we expect
		clickhouseMessagesMutex.Lock()
		clickhouseMessages = clickhouseMessages[:0]
		clickhouseMessagesMutex.Unlock()
		input := flowMessage("192.0.2.142", 434, 677)
		injectFlow(input, 0)
		time.Sleep(20 * time.Millisecond)

		// Check the flow was stored in clickhouseMessages
		expected := []*schema.FlowMessage{expectedFlowMessage("192.0.2.142", 434, 677)}
		clickhouseMessagesMutex.Lock()
		clickhouseMessagesCopy := make([]*schema.FlowMessage, len(clickhouseMessages))
		copy(clickhouseMessagesCopy, clickhouseMessages)
		clickhouseMessagesMutex.Unlock()
		if diff := helpers.Diff(clickhouseMessagesCopy, expected); diff != "" {
			t.Fatalf("Flow message (-got, +want):\n%s", diff)
		}

		// Try to inject a message with missing sampling rate
		input = flowMessage("192.0.2.142", 434, 677)
		input.SamplingRate = 0
		injectFlow(input, 0)
		time.Sleep(20 * time.Millisecond)

		gotMetrics = r.GetMetrics("akvorado_outlet_core_", "classifier_", "-flows_processing_", "flows_", "forwarded_", "received_")
		expectedMetrics = map[string]string{
			`classifier_exporter_cache_items_total`:                                    "0",
			`classifier_interface_cache_items_total`:                                   "0",
			`flows_errors_total{error="sampling rate missing",exporter="192.0.2.142"}`: "1",
			`received_flows_total{exporter="192.0.2.142"}`:                             "3",
			`received_flows_total{exporter="192.0.2.143"}`:                             "1",
			`forwarded_flows_total{exporter="192.0.2.142"}`:                            "2",
			`forwarded_flows_total{exporter="192.0.2.143"}`:                            "1",
			`flows_http_clients`:                                                       "0",
			`received_raw_flows_total`:                                                 "4",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}
	})

	// Test rate limiting
	t.Run("rate limiting", func(t *testing.T) {
		clickhouseMessagesMutex.Lock()
		clickhouseMessages = clickhouseMessages[:0]
		clickhouseMessagesMutex.Unlock()

		// Inject 20 flows with a rate limit of 100 (burst = 10).
		// The first 10 should be allowed, the remaining 10 should be rate limited.
		for range 20 {
			injectFlow(flowMessage("192.0.2.144", 434, 677), 100)
		}
		time.Sleep(50 * time.Millisecond)

		gotMetrics := r.GetMetrics("akvorado_outlet_core_",
			"received_flows_total", "forwarded_flows_total",
			"flows_rate_limited_total", "-flows_processing_")
		expectedMetrics := map[string]string{
			`received_flows_total{exporter="192.0.2.142"}`:     "3",
			`received_flows_total{exporter="192.0.2.143"}`:     "1",
			`received_flows_total{exporter="192.0.2.144"}`:     "20",
			`forwarded_flows_total{exporter="192.0.2.142"}`:    "2",
			`forwarded_flows_total{exporter="192.0.2.143"}`:    "1",
			`forwarded_flows_total{exporter="192.0.2.144"}`:    "10",
			`flows_rate_limited_total{exporter="192.0.2.144"}`: "10",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		clickhouseMessagesMutex.Lock()
		clickhouseMessagesLen := len(clickhouseMessages)
		clickhouseMessagesMutex.Unlock()
		if diff := helpers.Diff(clickhouseMessagesLen, 10); diff != "" {
			t.Fatalf("ClickHouse messages count (-got, +want):\n%s", diff)
		}
	})

	// Test HTTP flow clients (JSON)
	t.Run("http flows", func(t *testing.T) {
		c.httpFlowFlushDelay = 20 * time.Millisecond

		resp, err := http.Get(fmt.Sprintf("http://%s/api/v0/outlet/flows", c.d.HTTP.LocalAddr()))
		if err != nil {
			t.Fatalf("GET /api/v0/outlet/flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /api/v0/outlet/flows status code %d", resp.StatusCode)
		}

		// Metrics should tell we have a client
		gotMetrics := r.GetMetrics("akvorado_outlet_core_", "flows_http_clients", "-flows_processing_")
		expectedMetrics := map[string]string{
			`flows_http_clients`: "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Produce some flows
		clickhouseMessagesMutex.Lock()
		clickhouseMessages = clickhouseMessages[:0]
		clickhouseMessagesMutex.Unlock()
		for range 12 {
			injectFlow(flowMessage("192.0.2.142", 434, 677), 0)
		}

		// Wait for flows to be processed
		time.Sleep(100 * time.Millisecond)

		// Should have 12 flows in clickhouseMessages
		clickhouseMessagesMutex.Lock()
		clickhouseMessagesLen := len(clickhouseMessages)
		clickhouseMessagesMutex.Unlock()
		if clickhouseMessagesLen != 12 {
			t.Fatalf("Expected 12 flows in clickhouseMessages, got %d", clickhouseMessagesLen)
		}

		// Decode some of them
		reader := bufio.NewReader(resp.Body)
		decoder := json.NewDecoder(reader)
		for range 10 {
			var got helpers.M
			if err := decoder.Decode(&got); err != nil {
				t.Fatalf("GET /api/v0/outlet/flows error while reading body:\n%+v", err)
			}
			expected := helpers.M{
				"TimeReceived":    float64(200),
				"SamplingRate":    float64(1000),
				"ExporterAddress": "::ffff:192.0.2.142",
				"SrcAddr":         "::ffff:67.43.156.77",
				"DstAddr":         "::ffff:2.125.160.216",
				"SrcAS":           float64(0), // no geoip enrich anymore
				"InIf":            float64(434),
				"OutIf":           float64(677),
				"NextHop":         "",
				"SrcNetMask":      float64(0),
				"DstNetMask":      float64(0),
				"SrcVlan":         float64(0),
				"DstVlan":         float64(0),
				"DstAS":           float64(0),
				"OtherColumns": map[string]any{
					"ExporterName":     "192_0_2_142",
					"InIfName":         "Gi0/0/434",
					"OutIfName":        "Gi0/0/677",
					"InIfDescription":  "Interface 434",
					"OutIfDescription": "Interface 677",
					"InIfSpeed":        float64(1000),
					"OutIfSpeed":       float64(1000),
					"Bytes":            float64(6765),
					"Packets":          float64(4),
					"SrcPort":          float64(8534),
					"DstPort":          float64(80),
					"EType":            float64(2048),
					"Proto":            float64(6),
				},
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("GET /api/v0/outlet/flows (-got, +want):\n%s", diff)
			}
		}
	})

	// Test HTTP flow clients with a limit
	time.Sleep(10 * time.Millisecond)
	t.Run("http flows with limit", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/api/v0/outlet/flows?limit=4", c.d.HTTP.LocalAddr()))
		if err != nil {
			t.Fatalf("GET /api/v0/outlet/flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /api/v0/outlet/flows status code %d", resp.StatusCode)
		}

		// Metrics should tell we have a client
		gotMetrics := r.GetMetrics("akvorado_outlet_core_", "flows_http_clients")
		expectedMetrics := map[string]string{
			`flows_http_clients`: "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Produce some flows
		clickhouseMessagesMutex.Lock()
		clickhouseMessages = clickhouseMessages[:0]
		clickhouseMessagesMutex.Unlock()
		for range 12 {
			injectFlow(flowMessage("192.0.2.142", 434, 677), 0)
		}

		// Wait for flows to be processed
		time.Sleep(100 * time.Millisecond)

		// Should have 12 flows in clickhouseMessages
		clickhouseMessagesMutex.Lock()
		clickhouseMessagesLen := len(clickhouseMessages)
		clickhouseMessagesMutex.Unlock()
		if clickhouseMessagesLen != 12 {
			t.Fatalf("Expected 12 flows in clickhouseMessages, got %d", clickhouseMessagesLen)
		}

		// Check we got only 4
		reader := bufio.NewReader(resp.Body)
		count := 0
		for {
			_, err := reader.ReadString('\n')
			if err == io.EOF {
				t.Log("EOF")
				break
			}
			if err != nil {
				t.Fatalf("GET /api/v0/outlet/flows error while reading:\n%+v", err)
			}
			count++
			if count > 4 {
				break
			}
		}
		if count != 4 {
			t.Fatalf("GET /api/v0/outlet/flows got less than 4 flows (%d)", count)
		}
	})
}
