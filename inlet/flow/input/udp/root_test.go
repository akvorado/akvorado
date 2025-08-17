// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"net"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func TestUDPInput(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"

	done := make(chan bool)
	expected := &pb.RawFlow{
		SourceAddress: net.ParseIP("127.0.0.1").To16(),
		Payload:       []byte("hello world!"),
	}
	send := func(_ string, got *pb.RawFlow) {
		expected.TimeReceived = got.TimeReceived

		delta := uint64(time.Now().UTC().Unix()) - got.TimeReceived
		if delta > 1 {
			t.Errorf("TimeReceived out of range: %d (now: %d)", got.TimeReceived, time.Now().UTC().Unix())
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Input data (-got, +want):\n%s", diff)
		}

		// Check metrics
		gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_udp_")
		expectedMetrics := map[string]string{
			`bytes_total{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:                "12",
			`packets_total{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:              "1",
			`in_dropped_packets_total{listener="127.0.0.1:0",worker="0"}`:                        "0",
			`size_bytes_count{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:           "1",
			`size_bytes_sum{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:             "12",
			`size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.5"}`:  "12",
			`size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.9"}`:  "12",
			`size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.99"}`: "12",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Input metrics (-got, +want):\n%s", diff)
		}

		close(done)
	}

	in, err := configuration.New(r, daemon.NewMock(t), send)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)

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
	select {
	case <-time.After(20 * time.Millisecond):
		t.Fatal("no decoded flows received")
	case <-done:
	}
}

func TestUDPReceiveBuffer(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	configuration.ReceiveBuffer = 100_000_000
	in, err := configuration.New(r, daemon.NewMock(t), func(string, *pb.RawFlow) {})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)

	// Useless, but we observe no error, despite the requested buffer being too
	// big. That's expected.
}
