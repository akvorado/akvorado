// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow"
	"akvorado/inlet/kafka"
	"akvorado/inlet/metadata"
	"akvorado/inlet/routing"
)

func TestCore(t *testing.T) {
	r := reporter.NewMock(t)

	// Prepare all components.
	daemonComponent := daemon.NewMock(t)
	metadataComponent := metadata.NewMock(t, r, metadata.DefaultConfiguration(),
		metadata.Dependencies{Daemon: daemonComponent})
	flowComponent := flow.NewMock(t, r, flow.DefaultConfiguration())
	kafkaComponent, kafkaProducer := kafka.NewMock(t, r, kafka.DefaultConfiguration())
	httpComponent := httpserver.NewMock(t, r)
	routingComponent := routing.NewMock(t, r)
	routingComponent.PopulateRIB(t)

	// Instantiate and start core
	sch := schema.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon:   daemonComponent,
		Flow:     flowComponent,
		Metadata: metadataComponent,
		Kafka:    kafkaComponent,
		HTTP:     httpComponent,
		Routing:  routingComponent,
		Schema:   sch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	flowMessage := func(exporter string, in, out uint32) *schema.FlowMessage {
		msg := &schema.FlowMessage{
			TimeReceived:    200,
			SamplingRate:    1000,
			ExporterAddress: netip.MustParseAddr(exporter),
			InIf:            in,
			OutIf:           out,
			SrcAddr:         netip.MustParseAddr("67.43.156.77"),
			DstAddr:         netip.MustParseAddr("2.125.160.216"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:   6765,
				schema.ColumnPackets: 4,
				schema.ColumnEType:   0x800,
				schema.ColumnProto:   6,
				schema.ColumnSrcPort: 8534,
				schema.ColumnDstPort: 80,
			},
		}
		for k, v := range msg.ProtobufDebug {
			vi := v.(int)
			sch.ProtobufAppendVarint(msg, k, uint64(vi))
		}
		return msg
	}

	expectedFlowMessage := func(exporter string, in, out uint32) *schema.FlowMessage {
		expected := flowMessage(exporter, in, out)
		expected.SrcAS = 0 // no geoip enrich anymore
		expected.DstAS = 0 // no geoip enrich anymore
		expected.InIf = 0  // not serialized
		expected.OutIf = 0 // not serialized
		expected.ExporterAddress = netip.AddrFrom16(expected.ExporterAddress.As16())
		expected.SrcAddr = netip.AddrFrom16(expected.SrcAddr.As16())
		expected.DstAddr = netip.AddrFrom16(expected.DstAddr.As16())
		expected.ProtobufDebug[schema.ColumnInIfName] = fmt.Sprintf("Gi0/0/%d", in)
		expected.ProtobufDebug[schema.ColumnOutIfName] = fmt.Sprintf("Gi0/0/%d", out)
		expected.ProtobufDebug[schema.ColumnInIfDescription] = fmt.Sprintf("Interface %d", in)
		expected.ProtobufDebug[schema.ColumnOutIfDescription] = fmt.Sprintf("Interface %d", out)
		expected.ProtobufDebug[schema.ColumnInIfSpeed] = 1000
		expected.ProtobufDebug[schema.ColumnOutIfSpeed] = 1000
		expected.ProtobufDebug[schema.ColumnExporterName] = strings.ReplaceAll(exporter, ".", "_")
		return expected
	}

	t.Run("kafka", func(t *testing.T) {
		// Inject several messages with a cache miss from the SNMP
		// component for each of them. No message sent to Kafka.
		flowComponent.Inject(flowMessage("192.0.2.142", 434, 677))
		flowComponent.Inject(flowMessage("192.0.2.143", 434, 677))
		flowComponent.Inject(flowMessage("192.0.2.143", 437, 677))
		flowComponent.Inject(flowMessage("192.0.2.143", 434, 679))

		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_inlet_core_", "-flows_processing_")
		expectedMetrics := map[string]string{
			`classifier_exporter_cache_size_items`:                               "0",
			`classifier_interface_cache_size_items`:                              "0",
			`flows_errors_total{error="SNMP cache miss",exporter="192.0.2.142"}`: "1",
			`flows_errors_total{error="SNMP cache miss",exporter="192.0.2.143"}`: "3",
			`received_flows_total{exporter="192.0.2.142"}`:                       "1",
			`received_flows_total{exporter="192.0.2.143"}`:                       "3",
			`flows_http_clients`:                                                 "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Inject again the messages, this time, we will get a cache hit!
		kafkaProducer.ExpectInputAndSucceed()
		flowComponent.Inject(flowMessage("192.0.2.142", 434, 677))
		kafkaProducer.ExpectInputAndSucceed()
		flowComponent.Inject(flowMessage("192.0.2.143", 437, 679))

		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_inlet_core_", "classifier_", "-flows_processing_", "flows_", "received_", "forwarded_")
		expectedMetrics = map[string]string{
			`classifier_exporter_cache_size_items`:                               "0",
			`classifier_interface_cache_size_items`:                              "0",
			`flows_errors_total{error="SNMP cache miss",exporter="192.0.2.142"}`: "1",
			`flows_errors_total{error="SNMP cache miss",exporter="192.0.2.143"}`: "3",
			`received_flows_total{exporter="192.0.2.142"}`:                       "2",
			`received_flows_total{exporter="192.0.2.143"}`:                       "4",
			`forwarded_flows_total{exporter="192.0.2.142"}`:                      "1",
			`forwarded_flows_total{exporter="192.0.2.143"}`:                      "1",
			`flows_http_clients`:                                                 "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Now, check we get the message we expect
		input := flowMessage("192.0.2.142", 434, 677)
		received := make(chan bool)
		kafkaProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
			defer close(received)
			expectedTopic := fmt.Sprintf("flows-%s", sch.ProtobufMessageHash())
			if msg.Topic != expectedTopic {
				t.Errorf("Kafka message topic (-got, +want):\n-%s\n+%s", msg.Topic, expectedTopic)
			}

			b, err := msg.Value.Encode()
			if err != nil {
				t.Fatalf("Kafka message encoding error:\n%+v", err)
			}
			got := sch.ProtobufDecode(t, b)
			expected := expectedFlowMessage("192.0.2.142", 434, 677)
			if diff := helpers.Diff(&got, expected); diff != "" {
				t.Errorf("Kafka message (-got, +want):\n%s", diff)
			}

			return nil
		})
		flowComponent.Inject(input)
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("Kafka message not received")
		}

		// Try to inject a message with missing sampling rate
		input = flowMessage("192.0.2.142", 434, 677)
		input.SamplingRate = 0
		flowComponent.Inject(input)
		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_inlet_core_", "classifier_", "-flows_processing_", "flows_", "forwarded_", "received_")
		expectedMetrics = map[string]string{
			`classifier_exporter_cache_size_items`:                                     "0",
			`classifier_interface_cache_size_items`:                                    "0",
			`flows_errors_total{error="SNMP cache miss",exporter="192.0.2.142"}`:       "1",
			`flows_errors_total{error="SNMP cache miss",exporter="192.0.2.143"}`:       "3",
			`flows_errors_total{error="sampling rate missing",exporter="192.0.2.142"}`: "1",
			`received_flows_total{exporter="192.0.2.142"}`:                             "4",
			`received_flows_total{exporter="192.0.2.143"}`:                             "4",
			`forwarded_flows_total{exporter="192.0.2.142"}`:                            "2",
			`forwarded_flows_total{exporter="192.0.2.143"}`:                            "1",
			`flows_http_clients`: "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}
	})

	// Test the healthcheck function
	t.Run("healthcheck", func(t *testing.T) {
		got := r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["core"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckOK,
			Reason: "worker 0 ok",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}
	})

	// Test HTTP flow clients (JSON)
	t.Run("http flows", func(t *testing.T) {
		c.httpFlowFlushDelay = 20 * time.Millisecond

		resp, err := http.Get(fmt.Sprintf("http://%s/api/v0/inlet/flows", c.d.HTTP.LocalAddr()))
		if err != nil {
			t.Fatalf("GET /api/v0/inlet/flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /api/v0/inlet/flows status code %d", resp.StatusCode)
		}

		// Metrics should tell we have a client
		gotMetrics := r.GetMetrics("akvorado_inlet_core_", "flows_http_clients", "-flows_processing_")
		expectedMetrics := map[string]string{
			`flows_http_clients`: "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Produce some flows
		for range 12 {
			kafkaProducer.ExpectInputAndSucceed()
			flowComponent.Inject(flowMessage("192.0.2.142", 434, 677))
		}

		// Decode some of them
		reader := bufio.NewReader(resp.Body)
		decoder := json.NewDecoder(reader)
		for range 10 {
			var got gin.H
			if err := decoder.Decode(&got); err != nil {
				t.Fatalf("GET /api/v0/inlet/flows error while reading body:\n%+v", err)
			}
			expected := gin.H{
				"TimeReceived":    200,
				"SamplingRate":    1000,
				"ExporterAddress": "192.0.2.142",
				"SrcAddr":         "67.43.156.77",
				"DstAddr":         "2.125.160.216",
				"SrcAS":           0, // no geoip enrich anymore
				"InIf":            434,
				"OutIf":           677,

				"NextHop":    "",
				"SrcNetMask": 0,
				"DstNetMask": 0,
				"SrcVlan":    0,
				"DstVlan":    0,
				"GotASPath":  false,
				"DstAS":      0,
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("GET /api/v0/inlet/flows (-got, +want):\n%s", diff)
			}
		}
	})

	// Test HTTP flow clients with a limit
	time.Sleep(10 * time.Millisecond)
	t.Run("http flows with limit", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://%s/api/v0/inlet/flows?limit=4", c.d.HTTP.LocalAddr()))
		if err != nil {
			t.Fatalf("GET /api/v0/inlet/flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /api/v0/inlet/flows status code %d", resp.StatusCode)
		}

		// Metrics should tell we have a client
		gotMetrics := r.GetMetrics("akvorado_inlet_core_", "flows_http_clients")
		expectedMetrics := map[string]string{
			`flows_http_clients`: "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Produce some flows
		for range 12 {
			kafkaProducer.ExpectInputAndSucceed()
			flowComponent.Inject(flowMessage("192.0.2.142", 434, 677))
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
				t.Fatalf("GET /api/v0/inlet/flows error while reading:\n%+v", err)
			}
			count++
			if count > 4 {
				break
			}
		}
		if count != 4 {
			t.Fatalf("GET /api/v0/inlet/flows got less than 4 flows (%d)", count)
		}
	})

	// Test HTTP flow clients using protobuf
	time.Sleep(10 * time.Millisecond)
	t.Run("http flows with protobuf", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/api/v0/inlet/flows?limit=1", c.d.HTTP.LocalAddr()), nil)
		if err != nil {
			t.Fatalf("http.NewRequest() error:\n%+v", err)
		}
		req.Header.Set("accept", "application/x-protobuf")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v0/inlet/flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /api/v0/inlet/flows status code %d", resp.StatusCode)
		}

		// Produce some flows
		for range 12 {
			kafkaProducer.ExpectInputAndSucceed()
			flowComponent.Inject(flowMessage("192.0.2.142", 434, 677))
		}

		// Get the resulting flow
		reader := bufio.NewReader(resp.Body)
		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("GET /api/v0/inlet/flows error while reading:\n%+v", err)
		}
		t.Logf("got %v", got)

		// Decode
		sch := schema.NewMock(t)
		decoded := sch.ProtobufDecode(t, got)
		expected := expectedFlowMessage("192.0.2.142", 434, 677)
		if diff := helpers.Diff(decoded, expected); diff != "" {
			t.Errorf("HTTP message (-got, +want):\n%s", diff)
		}
	})
}
