// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	netHTTP "net/http"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/proto"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/inlet/bmp"
	"akvorado/inlet/flow"
	"akvorado/inlet/flow/decoder"
	"akvorado/inlet/geoip"
	"akvorado/inlet/kafka"
	"akvorado/inlet/snmp"
)

func TestCore(t *testing.T) {
	r := reporter.NewMock(t)

	// Prepare all components.
	daemonComponent := daemon.NewMock(t)
	snmpComponent := snmp.NewMock(t, r, snmp.DefaultConfiguration(), snmp.Dependencies{Daemon: daemonComponent})
	flowComponent := flow.NewMock(t, r, flow.DefaultConfiguration())
	geoipComponent := geoip.NewMock(t, r)
	kafkaComponent, kafkaProducer := kafka.NewMock(t, r, kafka.DefaultConfiguration())
	httpComponent := http.NewMock(t, r)
	bmpComponent, _ := bmp.NewMock(t, r, bmp.DefaultConfiguration())
	bmpComponent.PopulateRIB(t)

	// Instantiate and start core
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon: daemonComponent,
		Flow:   flowComponent,
		SNMP:   snmpComponent,
		GeoIP:  geoipComponent,
		Kafka:  kafkaComponent,
		HTTP:   httpComponent,
		BMP:    bmpComponent,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	flowMessage := func(exporter string, in, out uint32) *flow.Message {
		return &flow.Message{
			TimeReceived:    200,
			SequenceNum:     1000,
			SamplingRate:    1000,
			FlowDirection:   1,
			ExporterAddress: net.ParseIP(exporter),
			TimeFlowStart:   100,
			TimeFlowEnd:     200,
			Bytes:           6765,
			Packets:         4,
			InIf:            in,
			OutIf:           out,
			SrcAddr:         net.ParseIP("67.43.156.77"),
			DstAddr:         net.ParseIP("2.125.160.216"),
			Etype:           0x800,
			Proto:           6,
			SrcPort:         8534,
			DstPort:         80,
		}
	}

	t.Run("kafka", func(t *testing.T) {
		// Inject several messages with a cache miss from the SNMP
		// component for each of them. No message sent to Kafka.
		flowComponent.Inject(t, flowMessage("192.0.2.142", 434, 677))
		flowComponent.Inject(t, flowMessage("192.0.2.143", 434, 677))
		flowComponent.Inject(t, flowMessage("192.0.2.143", 437, 677))
		flowComponent.Inject(t, flowMessage("192.0.2.143", 434, 679))

		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_inlet_core_")
		expectedMetrics := map[string]string{
			`classifier_exporter_cache_size_items`:                         "0",
			`classifier_interface_cache_size_items`:                        "0",
			`flows_errors{error="SNMP cache miss",exporter="192.0.2.142"}`: "1",
			`flows_errors{error="SNMP cache miss",exporter="192.0.2.143"}`: "3",
			`flows_received{exporter="192.0.2.142"}`:                       "1",
			`flows_received{exporter="192.0.2.143"}`:                       "3",
			`flows_http_clients`:                                           "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Inject again the messages, this time, we will get a cache hit!
		kafkaProducer.ExpectInputAndSucceed()
		flowComponent.Inject(t, flowMessage("192.0.2.142", 434, 677))
		kafkaProducer.ExpectInputAndSucceed()
		flowComponent.Inject(t, flowMessage("192.0.2.143", 437, 679))

		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_inlet_core_", "classifier_", "flows_")
		expectedMetrics = map[string]string{
			`classifier_exporter_cache_size_items`:                         "0",
			`classifier_interface_cache_size_items`:                        "0",
			`flows_errors{error="SNMP cache miss",exporter="192.0.2.142"}`: "1",
			`flows_errors{error="SNMP cache miss",exporter="192.0.2.143"}`: "3",
			`flows_received{exporter="192.0.2.142"}`:                       "2",
			`flows_received{exporter="192.0.2.143"}`:                       "4",
			`flows_forwarded{exporter="192.0.2.142"}`:                      "1",
			`flows_forwarded{exporter="192.0.2.143"}`:                      "1",
			`flows_http_clients`:                                           "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Now, check we get the message we expect
		input := flowMessage("192.0.2.142", 434, 677)
		received := make(chan bool)
		kafkaProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
			defer close(received)
			expectedTopic := fmt.Sprintf("flows-v%d", flow.CurrentSchemaVersion)
			if msg.Topic != expectedTopic {
				t.Errorf("Kafka message topic (-got, +want):\n-%s\n+%s", msg.Topic, expectedTopic)
			}

			got := flow.Message{}
			b, err := msg.Value.Encode()
			if err != nil {
				t.Fatalf("Kafka message encoding error:\n%+v", err)
			}
			buf := proto.NewBuffer(b)
			err = buf.DecodeMessage(&got)
			if err != nil {
				t.Fatalf("Kakfa message decode error:\n%+v", err)
			}
			expected := flowMessage("192.0.2.142", 434, 677)
			expected.SrcAS = 35908
			expected.SrcCountry = "BT"
			expected.DstAS = 0 // not in database
			expected.DstCountry = "GB"
			expected.InIfName = "Gi0/0/434"
			expected.OutIfName = "Gi0/0/677"
			expected.InIfDescription = "Interface 434"
			expected.OutIfDescription = "Interface 677"
			expected.InIfSpeed = 1000
			expected.OutIfSpeed = 1000
			expected.ExporterName = "192_0_2_142"
			if diff := helpers.Diff(&got, expected); diff != "" {
				t.Errorf("Kafka message (-got, +want):\n%s", diff)
			}

			return nil
		})
		flowComponent.Inject(t, input)
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("Kafka message not received")
		}

		// Try to inject a message with missing sampling rate
		input = flowMessage("192.0.2.142", 434, 677)
		input.SamplingRate = 0
		flowComponent.Inject(t, input)
		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_inlet_core_", "classifier_", "flows_")
		expectedMetrics = map[string]string{
			`classifier_exporter_cache_size_items`:                               "0",
			`classifier_interface_cache_size_items`:                              "0",
			`flows_errors{error="SNMP cache miss",exporter="192.0.2.142"}`:       "1",
			`flows_errors{error="SNMP cache miss",exporter="192.0.2.143"}`:       "3",
			`flows_errors{error="sampling rate missing",exporter="192.0.2.142"}`: "1",
			`flows_received{exporter="192.0.2.142"}`:                             "4",
			`flows_received{exporter="192.0.2.143"}`:                             "4",
			`flows_forwarded{exporter="192.0.2.142"}`:                            "2",
			`flows_forwarded{exporter="192.0.2.143"}`:                            "1",
			`flows_http_clients`:                                                 "0",
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

	// Test HTTP flow clients
	t.Run("http flows", func(t *testing.T) {
		c.httpFlowFlushDelay = 20 * time.Millisecond

		resp, err := netHTTP.Get(fmt.Sprintf("http://%s/api/v0/inlet/flows", c.d.HTTP.LocalAddr()))
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
		for i := 0; i < 12; i++ {
			kafkaProducer.ExpectInputAndSucceed()
			flowComponent.Inject(t, flowMessage("192.0.2.142", 434, 677))
		}

		// Decode some of them
		reader := bufio.NewReader(resp.Body)
		decoder := json.NewDecoder(reader)
		for i := 0; i < 10; i++ {
			var got gin.H
			if err := decoder.Decode(&got); err != nil {
				t.Fatalf("GET /api/v0/inlet/flows error while reading body:\n%+v", err)
			}
			expected := gin.H{
				"TimeReceived":    200,
				"SequenceNum":     1000,
				"SamplingRate":    1000,
				"FlowDirection":   1,
				"ExporterAddress": "192.0.2.142",
				"TimeFlowStart":   100,
				"TimeFlowEnd":     200,
				"Bytes":           6765,
				"Packets":         4,
				"InIf":            434,
				"OutIf":           677,
				"SrcAddr":         "67.43.156.77",
				"DstAddr":         "2.125.160.216",
				"Etype":           0x800,
				"Proto":           6,
				"SrcPort":         8534,
				"DstPort":         80,
				// Added info
				"InIfDescription":  "Interface 434",
				"InIfName":         "Gi0/0/434",
				"OutIfDescription": "Interface 677",
				"OutIfName":        "Gi0/0/677",
				"InIfSpeed":        1000,
				"OutIfSpeed":       1000,
				"InIfBoundary":     "UNDEFINED",
				"OutIfBoundary":    "UNDEFINED",
				"DstCountry":       "GB",
				"SrcCountry":       "BT",
				"SrcAS":            35908,
				"ExporterName":     "192_0_2_142",
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("GET /api/v0/inlet/flows (-got, +want):\n%s", diff)
			}
		}
	})

	// Test HTTP flow clients with a limit
	time.Sleep(10 * time.Millisecond)
	t.Run("http flows with limit", func(t *testing.T) {
		resp, err := netHTTP.Get(fmt.Sprintf("http://%s/api/v0/inlet/flows?limit=4", c.d.HTTP.LocalAddr()))
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
		for i := 0; i < 12; i++ {
			kafkaProducer.ExpectInputAndSucceed()
			flowComponent.Inject(t, flowMessage("192.0.2.142", 434, 677))
		}

		// Check we got only 4
		reader := bufio.NewReader(resp.Body)
		count := 0
		for {
			_, err := reader.ReadString('\n')
			if err == io.EOF {
				fmt.Println("EOF")
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

	// Test HTTP flow clients using Protobuf
	t.Run("http flows", func(t *testing.T) {
		c.httpFlowFlushDelay = 20 * time.Millisecond

		client := netHTTP.Client{}
		req, err := netHTTP.NewRequest("GET", fmt.Sprintf("http://%s/api/v0/inlet/flows?limit=1", c.d.HTTP.LocalAddr()), nil)
		if err != nil {
			t.Fatalf("GET /api/v0/inlet/flows:\n%+v", err)
		}
		req.Header.Set("Accept", "application/x-protobuf")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v0/inlet/flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /api/v0/inlet/flows status code %d", resp.StatusCode)
		}

		// Produce one flow
		kafkaProducer.ExpectInputAndSucceed()
		flowComponent.Inject(t, flowMessage("192.0.2.142", 434, 677))

		// Decode it
		raw, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll() error:\n%+v", err)
		}
		var flow decoder.FlowMessage
		buf := proto.NewBuffer(raw)
		if err := buf.DecodeMessage(&flow); err != nil {
			t.Fatalf("DecodeMessage() error:\n%+v", err)
		}
	})

}
