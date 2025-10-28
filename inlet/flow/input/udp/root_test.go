// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"sync"
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
		gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_udp_", "-buffer_size", "-ebpf_loaded")
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
	case <-time.After(time.Second):
		t.Fatal("no decoded flows received")
	case <-done:
	}
}

func TestUDPReceiveBuffer(t *testing.T) {
	// Without setting receive buffer
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	in, err := configuration.New(r, daemon.NewMock(t), func(string, *pb.RawFlow) {})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)
	gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_udp_", "buffer_size")
	bufferSize := gotMetrics[`buffer_size_bytes{listener="127.0.0.1:0",worker="0"}`]
	bufferSize1, _ := strconv.ParseFloat(bufferSize, 32)

	// While setting receive buffer
	r = reporter.NewMock(t)
	configuration = DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	configuration.ReceiveBuffer = 100_000_000
	in, err = configuration.New(r, daemon.NewMock(t), func(string, *pb.RawFlow) {})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)
	gotMetrics = r.GetMetrics("akvorado_inlet_flow_input_udp_", "buffer_size")
	bufferSize = gotMetrics[`buffer_size_bytes{listener="127.0.0.1:0",worker="0"}`]
	bufferSize2, _ := strconv.ParseFloat(bufferSize, 32)

	if bufferSize2 < bufferSize1 {
		t.Fatalf("Buffer size was unchanged (%f <= %f)", bufferSize1, bufferSize2)
	}
}

func TestUDPWorkerBalancing(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	configuration.Workers = 16

	var wg sync.WaitGroup
	wg.Add(112)
	done := make(chan bool)
	go func() {
		wg.Wait()
		close(done)
	}()
	in, err := configuration.New(r, daemon.NewMock(t), func(string, *pb.RawFlow) {
		wg.Done()
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)

	// Connect and send many "flows"
	conn, err := net.Dial("udp", in.(*Input).address.String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}
	for range 112 {
		if _, err := conn.Write([]byte("hello world!")); err != nil {
			t.Fatalf("Write() error:\n%+v", err)
		}
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	ebpf := false
	for _, val := range r.GetMetrics("akvorado_inlet_flow_input_udp_", "ebpf_loaded") {
		if val == "1" {
			ebpf = true
		}
	}

	gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_udp_", "packets_total")
	if os.Getenv("CI_AKVORADO_EBPF") == "" && !ebpf {
		// Only one worker should have handled the 112 packets
		var worker string
		for m := range gotMetrics {
			r := regexp.MustCompile(`worker="(\d+)"`)
			worker = r.FindString(m)
			break
		}
		expectedMetrics := map[string]string{
			fmt.Sprintf(`packets_total{exporter="127.0.0.1",listener="127.0.0.1:0",%s}`, worker): "112",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Input metrics without (-got, +want):\n%s", diff)
		}
	} else {
		// Each worker should have handled exactly 7 packets. However, since the
		// counter is per CPU, this may not be 100% true. Be more permissive.
		expectedMetrics := map[string]string{}
		for worker := range 16 {
			key := fmt.Sprintf(`packets_total{exporter="127.0.0.1",listener="127.0.0.1:0",worker="%d"}`, worker)
			got, _ := strconv.Atoi(gotMetrics[key])
			if got > 20 || got < 2 {
				expectedMetrics[key] = "7"
			} else {
				expectedMetrics[key] = gotMetrics[key]
			}
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Input metrics with eBPF (-got, +want):\n%s", diff)
		}
	}
}
