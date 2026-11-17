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

// scriptedClickHouse returns workers flushing (or failing to) on each
// MaybeFlush call.
type scriptedClickHouse struct {
	flush     bool
	fail      bool
	finalized int
	maybe     int
}

func (c *scriptedClickHouse) NewWorker(_ int, bf *schema.FlowMessage) clickhouse.Worker {
	return &scriptedWorker{c: c, bf: bf}
}

type scriptedWorker struct {
	c  *scriptedClickHouse
	bf *schema.FlowMessage
}

func (w *scriptedWorker) Finalize() {
	w.bf.Finalize()
	w.c.finalized++
}

func (w *scriptedWorker) MaybeFlush(ctx context.Context) (clickhouse.WorkerStatus, bool) {
	w.c.maybe++
	if !w.c.flush {
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
		flush    bool
		fail     bool
		expected bool
	}{
		{"no flush", false, false, false},
		{"flush", true, false, true},
		{"failed flush", true, true, false},
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
			clickhouseComponent := &scriptedClickHouse{flush: tc.flush, fail: tc.fail}
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
			// Several flows, one flush attempt: only between records.
			if clickhouseComponent.finalized < 2 {
				t.Fatalf("Finalize() calls: got %d, want at least 2", clickhouseComponent.finalized)
			}
			if diff := helpers.Diff(clickhouseComponent.maybe, 1); diff != "" {
				t.Fatalf("MaybeFlush() calls (-got, +want):\n%s", diff)
			}
			if diff := helpers.Diff(got, tc.expected); diff != "" {
				t.Fatalf("receive() (-got, +want):\n%s", diff)
			}
		})
	}
}
