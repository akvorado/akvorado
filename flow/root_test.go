package flow

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
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

	// Send an option template
	template, err := ioutil.ReadFile(filepath.Join("testdata", "options-template-257.data"))
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
			t.Fatalf("After sending option template, received a flow while we should not:\n%v", flow)
		case <-time.After(30 * time.Millisecond):
			break out1
		}
	}

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_flow_nf_")
	expectedMetrics := map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "1",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send option data
	data, err := ioutil.ReadFile(filepath.Join("testdata", "options-data-257.data"))
	if err != nil {
		panic(err)
	}

	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}
out2:
	for {
		select {
		case flow := <-c.Flows():
			t.Fatalf("After sending option flowset, received a flow while we should not:\n%v", flow)
		case <-time.After(30 * time.Millisecond):
			break out2
		}
	}

	// Check metrics
	fmt.Printf("%+v\n", c.templates["127.0.0.1"].templates)
	gotMetrics = r.GetMetrics("akvorado_flow_nf_")
	expectedMetrics = map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "2",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send a regular template
	template, err = ioutil.ReadFile(filepath.Join("testdata", "template-260.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(template); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}
out3:
	for {
		select {
		case flow := <-c.Flows():
			t.Fatalf("After sending template, received a flow while we should not:\n%v", flow)
		case <-time.After(30 * time.Millisecond):
			break out3
		}
	}

	// Check metrics
	gotMetrics = r.GetMetrics("akvorado_flow_nf_")
	expectedMetrics = map[string]string{
		`count{sampler="127.0.0.1",version="9"}`:                                                                       "3",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                           "1",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                               "4",
		`flowset_records_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                  "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:                                   "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:                                       "1",
		`flowset_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                                          "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="257",type="options_template",version="9"}`: "1",
		`templates_count{obs_domain_id="0",sampler="127.0.0.1",template_id="260",type="template",version="9"}`:         "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after template (-got, +want):\n%s", diff)
	}

	// Send data
	data, err = ioutil.ReadFile(filepath.Join("testdata", "data-260.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}
	expectedFlows := []*FlowMessage{
		{
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1500,
			Packets:          1,
			SrcAddr:          net.ParseIP("198.38.121.178").To16(),
			DstAddr:          net.ParseIP("91.170.143.87").To16(),
			SrcNet:           24,
			DstNet:           14,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          19624,
			InIf:             335,
			OutIf:            450,
			ForwardingStatus: 64,
			TCPFlags:         16,
		}, {
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1500,
			Packets:          1,
			SrcAddr:          net.ParseIP("198.38.121.219").To16(),
			DstAddr:          net.ParseIP("88.122.57.97").To16(),
			SrcNet:           24,
			DstNet:           14,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          2444,
			InIf:             335,
			OutIf:            452,
			ForwardingStatus: 64,
			TCPFlags:         16,
		}, {
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1400,
			Packets:          1,
			SrcAddr:          net.ParseIP("173.194.190.106").To16(),
			DstAddr:          net.ParseIP("37.165.129.20").To16(),
			SrcNet:           20,
			DstNet:           18,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          53697,
			InIf:             461,
			OutIf:            306,
			ForwardingStatus: 64,
			TCPFlags:         16,
		}, {
			SequenceNum:      44797001,
			SamplerAddress:   net.ParseIP("127.0.0.1").To16(),
			SamplingRate:     30000,
			TimeFlowStart:    1647285926,
			TimeFlowEnd:      1647285926,
			Bytes:            1448,
			Packets:          1,
			SrcAddr:          net.ParseIP("74.125.100.234").To16(),
			DstAddr:          net.ParseIP("88.120.219.117").To16(),
			SrcNet:           16,
			DstNet:           14,
			Etype:            0x800,
			Proto:            6,
			SrcPort:          443,
			DstPort:          52300,
			InIf:             461,
			OutIf:            451,
			ForwardingStatus: 64,
			TCPFlags:         16,
		},
	}
	received := []*FlowMessage{}
out4:
	for {
		select {
		case flow := <-c.Flows():
			flow.TimeReceived = 0
			received = append(received, flow)
		case <-time.After(30 * time.Millisecond):
			break out4
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
		`count{sampler="127.0.0.1",version="9"}`:                                             "4",
		`flowset_records_sum{sampler="127.0.0.1",type="DataFlowSet",version="9"}`:            "4",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:     "4",
		`flowset_records_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`: "1",
		`flowset_records_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:        "1",
		`flowset_sum{sampler="127.0.0.1",type="DataFlowSet",version="9"}`:                    "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsDataFlowSet",version="9"}`:             "1",
		`flowset_sum{sampler="127.0.0.1",type="OptionsTemplateFlowSet",version="9"}`:         "1",
		`flowset_sum{sampler="127.0.0.1",type="TemplateFlowSet",version="9"}`:                "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
	}
}

func TestOutgoingChanFull(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	configuration.QueueSize = 1
	c := NewMock(t, r, configuration)
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
	template, err := ioutil.ReadFile(filepath.Join("testdata", "template-260.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(template); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}

	// Send data
	data, err := ioutil.ReadFile(filepath.Join("testdata", "data-260.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}

	checkQueueFullMetric := func(expected string) {
		gotMetrics := r.GetMetrics(
			"akvorado_flow_",
			"outgoing_queue_full_total",
		)
		expectedMetrics := map[string]string{
			`outgoing_queue_full_total`: expected,
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics after data (-got, +want):\n%s", diff)
		}
	}

	// We should receive 4 flows. The queue size is 1. So, the second flow is blocked.
	time.Sleep(30 * time.Millisecond)
	checkQueueFullMetric("1")

	// Accept the first flow and the third flow gets blocked too.
	select {
	case <-c.Flows():
	case <-time.After(30 * time.Millisecond):
		t.Fatal("First flow missing")
	}
	time.Sleep(30 * time.Millisecond)
	checkQueueFullMetric("2")

	// Accept the second flow and the fourth one gets blocked
	select {
	case <-c.Flows():
	case <-time.After(30 * time.Millisecond):
		t.Fatal("Second flow missing")
	}
	time.Sleep(30 * time.Millisecond)
	checkQueueFullMetric("3")

	// Accept the third flow and no more blocked flow
	select {
	case <-c.Flows():
	case <-time.After(30 * time.Millisecond):
		t.Fatal("Third flow missing")
	}
	time.Sleep(30 * time.Millisecond)
	checkQueueFullMetric("3")
}
