package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	netHTTP "net/http"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"

	"akvorado/daemon"
	"akvorado/flow"
	"akvorado/geoip"
	"akvorado/helpers"
	"akvorado/http"
	"akvorado/kafka"
	"akvorado/reporter"
	"akvorado/snmp"
)

func TestCore(t *testing.T) {
	r := reporter.NewMock(t)

	// Prepare all components.
	daemonComponent := daemon.NewMock(t)
	snmpComponent := snmp.NewMock(t, r, snmp.DefaultConfiguration, snmp.Dependencies{Daemon: daemonComponent})
	flowComponent := flow.NewMock(t, r, flow.DefaultConfiguration)
	geoipComponent := geoip.NewMock(t, r)
	kafkaComponent, kafkaProducer := kafka.NewMock(t, r, kafka.DefaultConfiguration)
	httpComponent := http.NewMock(t, r)

	// Instantiate and start core
	c, err := New(r, DefaultConfiguration, Dependencies{
		Daemon: daemonComponent,
		Flow:   flowComponent,
		Snmp:   snmpComponent,
		GeoIP:  geoipComponent,
		Kafka:  kafkaComponent,
		HTTP:   httpComponent,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	defer func() {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	flowMessage := func(sampler string, in, out uint32) *flow.FlowMessage {
		return &flow.FlowMessage{
			TimeReceived:   200,
			SequenceNum:    1000,
			SamplingRate:   1000,
			FlowDirection:  1,
			SamplerAddress: net.ParseIP(sampler),
			TimeFlowStart:  100,
			TimeFlowEnd:    200,
			Bytes:          6765,
			Packets:        4,
			InIf:           in,
			OutIf:          out,
			SrcAddr:        net.ParseIP("67.43.156.77"),
			DstAddr:        net.ParseIP("2.125.160.216"),
			Etype:          0x800,
			Proto:          6,
			SrcPort:        8534,
			DstPort:        80,
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
		gotMetrics := r.GetMetrics("akvorado_core_")
		expectedMetrics := map[string]string{
			`flows_errors{error="SNMP cache miss",sampler="192.0.2.142"}`: "1",
			`flows_errors{error="SNMP cache miss",sampler="192.0.2.143"}`: "3",
			`flows_received{sampler="192.0.2.142"}`:                       "1",
			`flows_received{sampler="192.0.2.143"}`:                       "3",
			`flows_http_clients`:                                          "0",
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
		gotMetrics = r.GetMetrics("akvorado_core_")
		expectedMetrics = map[string]string{
			`flows_errors{error="SNMP cache miss",sampler="192.0.2.142"}`: "1",
			`flows_errors{error="SNMP cache miss",sampler="192.0.2.143"}`: "3",
			`flows_received{sampler="192.0.2.142"}`:                       "2",
			`flows_received{sampler="192.0.2.143"}`:                       "4",
			`flows_forwarded{sampler="192.0.2.142"}`:                      "1",
			`flows_forwarded{sampler="192.0.2.143"}`:                      "1",
			`flows_http_clients`:                                          "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}

		// Now, check we get the message we expect
		input := flowMessage("192.0.2.142", 434, 677)
		kafkaProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
			if msg.Topic != "flows" {
				t.Errorf("Kafka message topic (-got, +want):\n-%s\n+%s", msg.Topic, "flows")
			}
			if msg.Key != sarama.StringEncoder("192.0.2.142") {
				t.Errorf("Kafka message key (-got, +want):\n-%s\n+%s", msg.Key, "192.0.2.142")
			}

			got := flow.FlowMessage{}
			b, err := msg.Value.Encode()
			if err != nil {
				t.Fatalf("Kafka message encoding error:\n%+v", err)
			}
			buf := proto.NewBuffer(b)
			err = buf.DecodeMessage(&got)
			if err != nil {
				t.Errorf("Kakfa message decode error:\n%+v", err)
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
			expected.SamplerName = "192_0_2_142"
			if diff := helpers.Diff(&got, expected); diff != "" {
				t.Errorf("Kafka message (-got, +want):\n%s", diff)
			}

			return nil
		})
		flowComponent.Inject(t, input)
		time.Sleep(20 * time.Millisecond)

		// Try to inject a message with missing sampling rate
		input = flowMessage("192.0.2.142", 434, 677)
		input.SamplingRate = 0
		flowComponent.Inject(t, input)
		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_core_")
		expectedMetrics = map[string]string{
			`flows_errors{error="SNMP cache miss",sampler="192.0.2.142"}`:       "1",
			`flows_errors{error="SNMP cache miss",sampler="192.0.2.143"}`:       "3",
			`flows_errors{error="sampling rate missing",sampler="192.0.2.142"}`: "1",
			`flows_received{sampler="192.0.2.142"}`:                             "4",
			`flows_received{sampler="192.0.2.143"}`:                             "4",
			`flows_forwarded{sampler="192.0.2.142"}`:                            "2",
			`flows_forwarded{sampler="192.0.2.143"}`:                            "1",
			`flows_http_clients`:                                                "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}
	})

	// Test the healthcheck endpoint
	t.Run("healthcheck", func(t *testing.T) {
		resp, err := netHTTP.Get(fmt.Sprintf("http://%s/healthcheck", c.d.HTTP.Address))
		if err != nil {
			t.Fatalf("GET /healthecheck:\n%+v", err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("GET /healthcheck: cannot read body:\n%+v", err)
		}
		if resp.StatusCode != 200 || string(body) != "ok" {
			t.Errorf("GET /healthcheck: got %d %q", resp.StatusCode, body)
		}
	})

	// Test HTTP flow clients
	t.Run("http flows", func(t *testing.T) {
		c.httpFlowFlushDelay = 20 * time.Millisecond

		resp, err := netHTTP.Get(fmt.Sprintf("http://%s/flows", c.d.HTTP.Address))
		if err != nil {
			t.Fatalf("GET /flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /flows status code %d", resp.StatusCode)
		}

		// Metrics should tell we have a client
		gotMetrics := r.GetMetrics("akvorado_core_", "flows_http_clients")
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
			var got map[string]interface{}
			if err := decoder.Decode(&got); err != nil {
				t.Fatalf("GET /flows error while reading body:\n%+v", err)
			}
			expected := map[string]interface{}{
				"TimeReceived":   200,
				"SequenceNum":    1000,
				"SamplingRate":   1000,
				"FlowDirection":  1,
				"SamplerAddress": "192.0.2.142",
				"TimeFlowStart":  100,
				"TimeFlowEnd":    200,
				"Bytes":          6765,
				"Packets":        4,
				"InIf":           434,
				"OutIf":          677,
				"SrcAddr":        "67.43.156.77",
				"DstAddr":        "2.125.160.216",
				"Etype":          0x800,
				"Proto":          6,
				"SrcPort":        8534,
				"DstPort":        80,
				// Added info
				"InIfDescription":  "Interface 434",
				"InIfName":         "Gi0/0/434",
				"OutIfDescription": "Interface 677",
				"OutIfName":        "Gi0/0/677",
				"DstCountry":       "GB",
				"SrcCountry":       "BT",
				"SrcAS":            35908,
				"SamplerName":      "192_0_2_142",
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("GET /flows (-got, +want):\n%s", diff)
			}
		}
	})

	// Test HTTP flow clients with a limit
	time.Sleep(10 * time.Millisecond)
	t.Run("http flows with limit", func(t *testing.T) {
		resp, err := netHTTP.Get(fmt.Sprintf("http://%s/flows?limit=4", c.d.HTTP.Address))
		if err != nil {
			t.Fatalf("GET /flows:\n%+v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("GET /flows status code %d", resp.StatusCode)
		}

		// Metrics should tell we have a client
		gotMetrics := r.GetMetrics("akvorado_core_", "flows_http_clients")
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
				break
			}
			if err != nil {
				t.Fatalf("GET /flows error while reading:\n%+v", err)
			}
			count++
			if count > 4 {
				break
			}
		}
		if count > 4 {
			t.Fatal("GET /flows got more than 4 flows")
		}
		if count != 4 {
			t.Fatalf("GET /flows got less than 4 flows (%d)", count)
		}
	})
}
