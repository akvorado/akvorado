// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/decoder"
)

func TestUDPInput(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	in, err := configuration.New(r, daemon.NewMock(t), &decoder.DummyDecoder{})
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
	var got []*schema.FlowMessage
	select {
	case got = <-ch:
		if len(got) == 0 {
			t.Fatalf("empty decoded flows received")
		}
	case <-time.After(20 * time.Millisecond):
		t.Fatal("no decoded flows received")
	}

	delta := uint64(time.Now().UTC().Unix()) - got[0].TimeReceived
	if delta > 1 {
		t.Errorf("TimeReceived out of range: %d (now: %d)", got[0].TimeReceived, time.Now().UTC().Unix())
	}
	expected := []*schema.FlowMessage{
		{
			TimeReceived:    got[0].TimeReceived,
			ExporterAddress: netip.MustParseAddr("::ffff:127.0.0.1"),
			ProtobufDebug: map[schema.ColumnKey]interface{}{
				schema.ColumnBytes:           12,
				schema.ColumnPackets:         1,
				schema.ColumnInIfDescription: []byte("hello world!"),
			},
		},
	}
	if diff := helpers.Diff(got, expected, helpers.DiffFormatter(reflect.TypeOf(schema.ColumnBytes), fmt.Sprint)); diff != "" {
		t.Fatalf("Input data (-got, +want):\n%s", diff)
	}

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_udp_")
	expectedMetrics := map[string]string{
		`bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:                              "12",
		`packets{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:                            "1",
		`in_drops{listener="127.0.0.1:0",worker="0"}`:                                                "0",
		`summary_size_bytes_count{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:           "1",
		`summary_size_bytes_sum{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:             "12",
		`summary_size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.5"}`:  "12",
		`summary_size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.9"}`:  "12",
		`summary_size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.99"}`: "12",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Input metrics (-got, +want):\n%s", diff)
	}
}

func TestOverflow(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration().(*Configuration)
	configuration.Listen = "127.0.0.1:0"
	configuration.QueueSize = 1
	in, err := configuration.New(r, daemon.NewMock(t), &decoder.DummyDecoder{})
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

	// Check metrics
	gotMetrics := r.GetMetrics("akvorado_inlet_flow_input_udp_")
	expectedMetrics := map[string]string{
		`bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:                              "120",
		`in_drops{listener="127.0.0.1:0",worker="0"}`:                                                "0",
		`out_drops{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:                          "9",
		`packets{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:                            "10",
		`summary_size_bytes_count{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:           "10",
		`summary_size_bytes_sum{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0"}`:             "120",
		`summary_size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.5"}`:  "12",
		`summary_size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.9"}`:  "12",
		`summary_size_bytes{exporter="127.0.0.1",listener="127.0.0.1:0",worker="0",quantile="0.99"}`: "12",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Input metrics (-got, +want):\n%s", diff)
	}
}
