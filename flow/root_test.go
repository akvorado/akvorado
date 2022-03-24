package flow

import (
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"
	"time"

	"akvorado/helpers"
	"akvorado/reporter"
)

func sendNetflowData(t *testing.T, conn net.Conn) {
	t.Helper()
	// Send template
	template, err := ioutil.ReadFile(filepath.Join("decoder", "netflow", "testdata", "template-260.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(template); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}

	// Send data
	data, err := ioutil.ReadFile(filepath.Join("decoder", "netflow", "testdata", "data-260.data"))
	if err != nil {
		panic(err)
	}
	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Write() failure:\n%+v", err)
	}
}

func TestNetflowProcessing(t *testing.T) {
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

	sendNetflowData(t, conn)

	// Receive flows
	received := []*Message{}
out:
	for {
		select {
		case flow := <-c.Flows():
			flow.TimeReceived = 0
			received = append(received, flow)
		case <-time.After(30 * time.Millisecond):
			break out
		}
	}

	if len(received) != 4 {
		t.Fatalf("Instead of receiving 4 flows, got %d flows", len(received))
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

	sendNetflowData(t, conn)

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
