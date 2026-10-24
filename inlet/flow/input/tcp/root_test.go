// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package tcp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/inlet/flow/decoder"
)

func netflowDecoder(t *testing.T) decoder.Decoder {
	t.Helper()
	dec, err := decoder.Get(pb.RawFlow_DECODER_NETFLOW)
	if err != nil {
		t.Fatalf("Get() error:\n%+v", err)
	}
	return dec
}

func TestTCPInputUnsupportedDecoder(t *testing.T) {
	r := reporter.NewMock(t)
	dec, err := decoder.Get(pb.RawFlow_DECODER_SFLOW)
	if err != nil {
		t.Fatalf("Get() error:\n%+v", err)
	}
	_, err = DefaultConfiguration().New(r, daemon.NewMock(t), dec, func(string, *pb.RawFlow) {})
	if err == nil {
		t.Fatal("New() did not error")
	}
}

func TestTCPInput(t *testing.T) {
	// The capture contains a template message (124 bytes) and a data message
	// (3730 bytes) split over two TCP segments.
	stream := helpers.ReadPcapL4(t, filepath.Join("testdata", "ipfix.pcap"))
	expected := []*pb.RawFlow{
		{
			SourceAddress: net.ParseIP("127.0.0.1").To16(),
			Payload:       stream[:124],
		}, {
			SourceAddress: net.ParseIP("127.0.0.1").To16(),
			Payload:       stream[124:],
		},
	}
	expectedMetrics := map[string]string{
		`bytes_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:              "3854",
		`closed_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`: "1",
		`connections{exporter="127.0.0.1",listener="127.0.0.1:0"}`:              "0",
		`errors_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:             "0",
		`messages_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:           "2",
		`opened_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`: "1",
	}

	for _, size := range []int{1, 2, 3, 4, 5, 7, 15, 16, 17, 100, 123, 124, 125, 1000, 1448, len(stream)} {
		t.Run(fmt.Sprintf("chunk size %d", size), func(t *testing.T) {
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration().(*Configuration)
			configuration.Listen = "127.0.0.1:0"

			var mu sync.Mutex
			got := []*pb.RawFlow{}
			send := func(exporter string, flow *pb.RawFlow) {
				mu.Lock()
				defer mu.Unlock()
				if exporter != "127.0.0.1" {
					t.Errorf("send() exporter: %q", exporter)
				}
				delta := uint64(time.Now().UTC().Unix()) - flow.TimeReceived
				if delta > 1 {
					t.Errorf("TimeReceived out of range: %d (now: %d)", flow.TimeReceived, time.Now().UTC().Unix())
				}
				got = append(got, &pb.RawFlow{
					SourceAddress: flow.SourceAddress,
					Payload:       bytes.Clone(flow.Payload),
				})
			}

			in, err := configuration.New(r, daemon.NewMock(t), netflowDecoder(t), send)
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, in)

			conn, err := net.Dial("tcp", in.(*Input).address.String())
			if err != nil {
				t.Fatalf("Dial() error:\n%+v", err)
			}
			for chunk := range slices.Chunk(stream, size) {
				if _, err := conn.Write(chunk); err != nil {
					t.Fatalf("Write() error:\n%+v", err)
				}
			}
			conn.Close()

			var diff, diffMetrics string
			for range 100 {
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				diff = helpers.Diff(got, expected)
				mu.Unlock()
				diffMetrics = helpers.Diff(r.GetMetrics("akvorado_inlet_flow_input_tcp_"), expectedMetrics)
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
		})
	}
}

func TestTCPInputErrors(t *testing.T) {
	cases := []struct {
		Description string
		Payload     []byte
		Errors      string
	}{
		{
			Description: "NetFlow v9 header",
			Payload:     []byte{0, 9, 0, 20},
			Errors:      "1",
		}, {
			Description: "length too small",
			Payload:     []byte{0, 10, 0, 4},
			Errors:      "1",
		}, {
			Description: "truncated message",
			Payload:     []byte{0, 10, 0, 20, 0, 0, 0, 0},
			Errors:      "1",
		}, {
			Description: "truncated header",
			Payload:     []byte{0, 10},
			Errors:      "1",
		}, {
			Description: "no data",
			Payload:     []byte{},
			Errors:      "0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration().(*Configuration)
			configuration.Listen = "127.0.0.1:0"
			in, err := configuration.New(r, daemon.NewMock(t), netflowDecoder(t), func(string, *pb.RawFlow) {
				t.Error("send() should not be called")
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, in)

			conn, err := net.Dial("tcp", in.(*Input).address.String())
			if err != nil {
				t.Fatalf("Dial() error:\n%+v", err)
			}
			defer conn.Close()
			if _, err := conn.Write(tc.Payload); err != nil {
				t.Fatalf("Write() error:\n%+v", err)
			}
			conn.(*net.TCPConn).CloseWrite()

			// Wait for the server to close the connection
			conn.SetReadDeadline(time.Now().Add(time.Second))
			if _, err := conn.Read(make([]byte, 1)); err != io.EOF {
				t.Fatalf("Read() error:\n%+v", err)
			}

			expectedMetrics := map[string]string{
				`closed_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`: "1",
				`connections{exporter="127.0.0.1",listener="127.0.0.1:0"}`:              "0",
				`errors_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`:             tc.Errors,
				`opened_connections_total{exporter="127.0.0.1",listener="127.0.0.1:0"}`: "1",
			}
			gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_tcp_", "errors_", "opened_", "closed_", "connections")
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				t.Errorf("Input metrics (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestTCPInputConnections(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	in, err := configuration.New(r, daemon.NewMock(t), netflowDecoder(t), func(string, *pb.RawFlow) {})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, in)

	waitFor := func(expected string) {
		t.Helper()
		expectedMetrics := map[string]string{
			`connections{exporter="127.0.0.1",listener="127.0.0.1:0"}`: expected,
		}
		var diff string
		for range 100 {
			diff = helpers.Diff(r.GetMetrics("akvorado_inlet_flow_input_tcp_", "connections"), expectedMetrics)
			if diff == "" {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("Input metrics (-got, +want):\n%s", diff)
	}

	conns := []net.Conn{}
	for range 2 {
		conn, err := net.Dial("tcp", in.(*Input).address.String())
		if err != nil {
			t.Fatalf("Dial() error:\n%+v", err)
		}
		conns = append(conns, conn)
	}
	waitFor("2")
	conns[0].Close()
	waitFor("1")
	conns[1].Close()
	waitFor("0")
}
