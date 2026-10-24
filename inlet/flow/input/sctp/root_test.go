// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package sctp

import (
	"bytes"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/georgeyanev/go-sctp"
	"golang.org/x/sys/unix"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func skipWithoutSCTP(t *testing.T) {
	t.Helper()
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_SCTP)
	if err != nil {
		t.Skipf("SCTP not supported: %v", err)
	}
	unix.Close(fd)
}

func TestSCTPInput(t *testing.T) {
	skipWithoutSCTP(t)
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"

	var mu sync.Mutex
	got := [][]byte{}
	send := func(exporter string, flow *pb.RawFlow) {
		mu.Lock()
		defer mu.Unlock()
		if exporter != "127.0.0.1" {
			t.Errorf("send() exporter: %q", exporter)
		}
		if diff := helpers.Diff(flow.SourceAddress, []byte(net.ParseIP("127.0.0.1").To16())); diff != "" {
			t.Errorf("send() source address (-got, +want):\n%s", diff)
		}
		got = append(got, bytes.Clone(flow.Payload))
	}
	in, err := configuration.New(r, daemon.NewMock(t), nil, send)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)

	conn, err := sctp.Dial("sctp", in.(*Input).address.String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}
	// Wait for the connection to be accepted. Otherwise, the remote address may
	// not be available.
	for range 100 {
		if diff := helpers.Diff(r.GetMetrics("akvorado_inlet_flow_input_sctp_", "opened_connections_total"), map[string]string{
			`opened_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`: "1",
		}); diff == "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The capture contains a template message (124 bytes) and a data message
	// (3730 bytes).
	stream := helpers.ReadPcapL4(t, filepath.Join("..", "tcp", "testdata", "ipfix.pcap"))
	large := bytes.Repeat([]byte("a"), maxMessageSize)
	tooLong := bytes.Repeat([]byte("b"), maxMessageSize+1)
	for _, message := range [][]byte{stream[:124], stream[124:], large, tooLong, stream[:124]} {
		if _, err := conn.(*sctp.SCTPConn).WriteMsg(message, nil); err != nil {
			t.Fatalf("WriteMsg() error:\n%+v", err)
		}
	}
	conn.Close()

	expected := [][]byte{stream[:124], stream[124:], large, stream[:124]}
	expectedMetrics := map[string]string{
		`bytes_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:                              "69513",
		`closed_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:                 "1",
		`connections{exporter="127.0.0.1",listener="127.0.0.1:0"}`:                              "0",
		`dropped_messages_total{exporter="127.0.0.1",listener="127.0.0.1:0",reason="aborted"}`:  "0",
		`dropped_messages_total{exporter="127.0.0.1",listener="127.0.0.1:0",reason="too-long"}`: "1",
		`errors_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:                             "0",
		`messages_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:                           "4",
		`opened_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:                 "1",
	}
	var diff, diffMetrics string
	for range 100 {
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		diff = helpers.Diff(got, expected)
		mu.Unlock()
		diffMetrics = helpers.Diff(r.GetMetrics("akvorado_inlet_flow_input_sctp_"), expectedMetrics)
		if diff == "" && diffMetrics == "" {
			break
		}
	}
	if diff != "" {
		t.Errorf("Input data (-got, +want):\n%s", diff)
	}
	if diffMetrics != "" {
		t.Errorf("Input metrics (-got, +want):\n%s", diffMetrics)
	}
}

func TestSCTPInputStop(t *testing.T) {
	skipWithoutSCTP(t)
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	in, err := configuration.New(r, daemon.NewMock(t), nil, func(string, *pb.RawFlow) {})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := in.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}

	// Stopping with an open connection should not hang.
	conn, err := sctp.Dial("sctp", in.(*Input).address.String())
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}
	defer conn.Close()
	expectedMetrics := map[string]string{
		`connections{exporter="127.0.0.1",listener="127.0.0.1:0"}`: "1",
	}
	var diff string
	for range 100 {
		diff = helpers.Diff(r.GetMetrics("akvorado_inlet_flow_input_sctp_", "connections"), expectedMetrics)
		if diff == "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if diff != "" {
		t.Fatalf("Input metrics (-got, +want):\n%s", diff)
	}

	done := make(chan error)
	go func() {
		done <- in.Stop()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return")
	}
}
