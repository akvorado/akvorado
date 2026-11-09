// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/flow"
	"akvorado/outlet/kafkainput"
	"akvorado/outlet/metadata"
	"akvorado/outlet/routing"
)

// scriptedClickHouse returns workers flushing on the given FinalizeAndSend calls.
type scriptedClickHouse struct {
	flushOn func(call int) bool
	fail    bool
	calls   int
}

func (c *scriptedClickHouse) NewWorker(_ int, bf *schema.FlowMessage) clickhouse.Worker {
	return &scriptedWorker{c: c, bf: bf}
}

type scriptedWorker struct {
	c  *scriptedClickHouse
	bf *schema.FlowMessage
}

func (w *scriptedWorker) FinalizeAndSend(ctx context.Context) (clickhouse.WorkerStatus, bool) {
	w.bf.Finalize()
	w.c.calls++
	if !w.c.flushOn(w.c.calls) {
		return clickhouse.WorkerStatusIdle, false
	}
	return clickhouse.WorkerStatusSteady, w.Flush(ctx) == nil
}

func (w *scriptedWorker) Flush(context.Context) error {
	if w.c.fail {
		return errors.New("ClickHouse unavailable")
	}
	w.bf.Clear()
	return nil
}

func (w *scriptedWorker) Close() {}

func TestWorkerFlushed(t *testing.T) {
	sflowData := helpers.ReadPcapL4(t, filepath.Join("..", "flow", "decoder", "sflow", "testdata", "data-1140.pcap"))
	rawFlow := &pb.RawFlow{
		TimeReceived:    uint64(time.Now().Unix()),
		Payload:         sflowData,
		SourceAddress:   net.ParseIP("127.0.0.1").To16(),
		Decoder:         pb.RawFlow_DECODER_SFLOW,
		TimestampSource: pb.RawFlow_TS_INPUT,
	}
	data, err := rawFlow.MarshalVT()
	if err != nil {
		t.Fatalf("MarshalVT() error:\n%+v", err)
	}

	cases := []struct {
		name     string
		flushOn  func(int) bool
		fail     bool
		expected kafkainput.Flushed
	}{
		{"no flush", func(int) bool { return false }, false, kafkainput.FlushedNothing},
		{"flush on each flow", func(int) bool { return true }, false, kafkainput.FlushedAll},
		{"flush on first flow", func(call int) bool { return call == 1 }, false, kafkainput.FlushedBefore},
		{"failed flush", func(int) bool { return true }, true, kafkainput.FlushedNothing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := reporter.NewMock(t)
			daemonComponent := daemon.NewMock(t)
			sch := schema.NewMock(t)
			flowComponent, err := flow.New(r, flow.DefaultConfiguration(), flow.Dependencies{Schema: sch})
			if err != nil {
				t.Fatalf("flow.New() error:\n%+v", err)
			}
			routingComponent := routing.NewMock(t, r)
			kafkaInputComponent, _ := kafkainput.NewMock(t, kafkainput.DefaultConfiguration())
			clickhouseComponent := &scriptedClickHouse{flushOn: tc.flushOn, fail: tc.fail}
			c, err := New(r, DefaultConfiguration(), Dependencies{
				Daemon: daemonComponent,
				Flow:   flowComponent,
				Metadata: metadata.NewMock(t, r, metadata.DefaultConfiguration(),
					metadata.Dependencies{Daemon: daemonComponent}),
				KafkaInput: kafkaInputComponent,
				ClickHouse: clickhouseComponent,
				HTTP:       httpserver.NewMock(t, r),
				Routing:    routingComponent,
				Schema:     sch,
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, c)

			receive, _, _ := c.newWorker(0, make(chan kafkainput.ScaleRequest, 100))
			got, err := receive(t.Context(), data)
			if err != nil {
				t.Fatalf("receive() error:\n%+v", err)
			}
			if clickhouseComponent.calls < 2 {
				t.Fatalf("FinalizeAndSend() calls: got %d, want at least 2", clickhouseComponent.calls)
			}
			if got != tc.expected {
				t.Fatalf("receive(): got %d, want %d", got, tc.expected)
			}
		})
	}
}
