package udp

import (
	"net"
	"testing"
	"time"

	"akvorado/daemon"
	"akvorado/flow/input"
	"akvorado/helpers"
	"akvorado/reporter"
)

func TestUDPInput(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	configuration.Listen = "127.0.0.1:0"
	in, err := configuration.New(r, daemon.NewMock(t))
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	ch, err := in.Start()
	if err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	defer func() {
		if err := in.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	// Connect
	conn, err := net.Dial("udp", in.(*Input).address.String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	// Send data
	if _, err := conn.Write([]byte("hello world!")); err != nil {
		t.Fatalf("Write() error:\n%+v", err)
	}

	// Get it back
	var got input.Flow
	select {
	case got = <-ch:
	case <-time.After(20 * time.Millisecond):
		t.Fatal("Input data missing")
	}

	delta := got.TimeReceived.Sub(time.Now())
	if delta > time.Second || delta < -time.Second {
		t.Errorf("TimeReceived out of range: %s (now: %s)", got.TimeReceived, time.Now())
	}
	expected := input.Flow{
		TimeReceived: got.TimeReceived,
		Payload:      []byte("hello world!"),
		Source:       net.ParseIP("127.0.0.1"),
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Input data (-got, +want):\n%s", diff)
	}

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_flow_input_udp_")
	expectedMetrics := map[string]string{
		`bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:                              "12",
		`packets{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:                            "1",
		`summary_size_bytes_count{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:           "1",
		`summary_size_bytes_sum{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:             "12",
		`summary_size_bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0",quantile="0.5"}`:  "12",
		`summary_size_bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0",quantile="0.9"}`:  "12",
		`summary_size_bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0",quantile="0.99"}`: "12",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Input metrics (-got, +want):\n%s", diff)
	}
}

func TestOverflow(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	configuration.Listen = "127.0.0.1:0"
	configuration.QueueSize = 1
	in, err := configuration.New(r, daemon.NewMock(t))
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	_, err = in.Start()
	if err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	defer func() {
		if err := in.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	// Connect
	conn, err := net.Dial("udp", in.(*Input).address.String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	// Send data
	for i := 0; i < 10; i++ {
		if _, err := conn.Write([]byte("hello world!")); err != nil {
			t.Fatalf("Write() error:\n%+v", err)
		}
	}
	time.Sleep(20 * time.Millisecond)

	// Check metrics (same as before because we got only one packet, others were dropped)
	gotMetrics := r.GetMetrics("akvorado_flow_input_udp_")
	expectedMetrics := map[string]string{
		`bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:                              "12",
		`drops{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:                              "9",
		`packets{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:                            "1",
		`summary_size_bytes_count{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:           "1",
		`summary_size_bytes_sum{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0"}`:             "12",
		`summary_size_bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0",quantile="0.5"}`:  "12",
		`summary_size_bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0",quantile="0.9"}`:  "12",
		`summary_size_bytes{listener="127.0.0.1:0",sampler="127.0.0.1",worker="0",quantile="0.99"}`: "12",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Input metrics (-got, +want):\n%s", diff)
	}
}
