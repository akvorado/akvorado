package flow

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	netHTTP "net/http"
	"path/filepath"
	"testing"
	"time"

	"akvorado/helpers"
	"akvorado/reporter"
)

var startUDPPort = rand.Intn(1000) + 22000

func TestDecoding(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration)
	defer func() {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()
	conn, err := net.Dial("udp", c.Address.String())
	if err != nil {
		t.Fatalf("Dial() failure:\n%+v", err)
	}

	// Send template
	template, err := ioutil.ReadFile(filepath.Join("testdata", "template.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(template); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}
out1:
	for {
		select {
		case flow := <-c.Flows():
			t.Fatalf("After sending templates, received a flow while we should not:\n%v", flow)
		case <-time.After(10 * time.Millisecond):
			break out1
		}
	}

	// Check templates (with metrics)
	gotMetrics := r.GetMetrics("akvorado_flow_nf_")
	expectedMetrics := map[string]string{
		`count{router="127.0.0.1",version="9"}`:                                                               "1",
		`flowset_records_sum{router="127.0.0.1",type="TemplateFlowSet",version="9"}`:                          "1",
		`flowset_sum{router="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`templates_count{obs_domain_id="0",router="127.0.0.1",template_id="266",type="template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send data
	data, err := ioutil.ReadFile(filepath.Join("testdata", "flow.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}
	expectedFlows := []*FlowMessage{
		{
			SequenceNum:      21029551,
			SamplerAddress:   net.ParseIP("127.0.0.1").To4(),
			TimeFlowStart:    1646339556,
			TimeFlowEnd:      1646339556,
			Bytes:            1500,
			Packets:          1,
			SrcAddr:          net.ParseIP("2a02:26f0:b1::5c7a:5d0b"),
			DstAddr:          net.ParseIP("2a01:e0a:444:c640:d48e:9641:b07:1bed"),
			SrcNet:           48,
			DstNet:           52,
			Etype:            34525,
			Proto:            6,
			SrcPort:          443,
			DstPort:          38120,
			InIf:             461,
			OutIf:            334,
			ForwardingStatus: 64,
			TCPFlags:         16,
			IPv6FlowLabel:    795690,
		}, {
			SequenceNum:      21029551,
			SamplerAddress:   net.ParseIP("127.0.0.1").To4(),
			TimeFlowStart:    1646339556,
			TimeFlowEnd:      1646339556,
			Bytes:            1280,
			Packets:          1,
			SrcAddr:          net.ParseIP("2a00:1450:4007:4::b"),
			DstAddr:          net.ParseIP("2a01:e0a:85f:13f0:e01a:dfa1:6556:4786"),
			SrcNet:           48,
			DstNet:           52,
			Etype:            34525,
			Proto:            6,
			SrcPort:          443,
			DstPort:          54496,
			InIf:             461,
			OutIf:            334,
			ForwardingStatus: 64,
			TCPFlags:         24,
			IPv6FlowLabel:    190180,
		}, {
			SequenceNum:      21029551,
			SamplerAddress:   net.ParseIP("127.0.0.1").To4(),
			TimeFlowStart:    1646339556,
			TimeFlowEnd:      1646339556,
			Bytes:            1378,
			Packets:          1,
			SrcAddr:          net.ParseIP("2a00:1450:4007:2b::a"),
			DstAddr:          net.ParseIP("2a01:e0a:1dd:a1d0:8c19:1fc5:f427:2c13"),
			SrcNet:           48,
			DstNet:           52,
			Etype:            34525,
			Proto:            17,
			SrcPort:          443,
			DstPort:          37867,
			InIf:             461,
			OutIf:            334,
			ForwardingStatus: 64,
		}, {
			SequenceNum:      21029551,
			SamplerAddress:   net.ParseIP("127.0.0.1").To4(),
			TimeFlowStart:    1646339556,
			TimeFlowEnd:      1646339556,
			Bytes:            1500,
			Packets:          1,
			SrcAddr:          net.ParseIP("2a00:86c0:121:121::207"),
			DstAddr:          net.ParseIP("2a01:e0a:929:dd80:3899:4413:7a11:da00"),
			SrcNet:           48,
			DstNet:           52,
			Etype:            34525,
			Proto:            6,
			SrcPort:          443,
			DstPort:          53396,
			InIf:             335,
			OutIf:            308,
			ForwardingStatus: 64,
			TCPFlags:         16,
		},
	}
	received := []*FlowMessage{}
out2:
	for {
		select {
		case flow := <-c.Flows():
			flow.TimeReceived = 0
			received = append(received, flow)
		case <-time.After(10 * time.Millisecond):
			break out2
		}
	}

	if diff := helpers.Diff(received, expectedFlows); diff != "" {
		t.Fatalf("After sending flows, received flows (-got, +want):\n%s", diff)
	}
	gotMetrics = r.GetMetrics(
		"akvorado_flow_nf_",
		"count",
		"flowset_records_sum",
		"flowset_sum",
	)
	expectedMetrics = map[string]string{
		`count{router="127.0.0.1",version="9"}`:                                      "2",
		`flowset_records_sum{router="127.0.0.1",type="DataFlowSet",version="9"}`:     "4",
		`flowset_records_sum{router="127.0.0.1",type="TemplateFlowSet",version="9"}`: "1",
		`flowset_sum{router="127.0.0.1",type="DataFlowSet",version="9"}`:             "1",
		`flowset_sum{router="127.0.0.1",type="TemplateFlowSet",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestServeProtoFile(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration)

	// Check the HTTP server is running and answering metrics
	resp, err := netHTTP.Get(fmt.Sprintf("http://%s/flow/flow.proto", c.d.HTTP.Address))
	if err != nil {
		t.Fatalf("GET /flow/flow.proto:\n%+v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /flow/flow.proto: got status code %d, not 200", resp.StatusCode)
	}
}
