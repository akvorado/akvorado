// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/kafka"
)

// worker represents a worker processing incoming flows.
type worker struct {
	c       *Component
	l       reporter.Logger
	cw      clickhouse.Worker
	bf      *schema.FlowMessage
	rawFlow pb.RawFlow
}

// newWorker instantiates a new worker and returns a callback function to
// process an incoming flow and a function to call on shutdown.
func (c *Component) newWorker(i int) (kafka.ReceiveFunc, kafka.ShutdownFunc) {
	bf := c.d.Schema.NewFlowMessage()
	w := worker{
		c:  c,
		l:  c.r.With().Int("worker", i).Logger(),
		bf: bf,
		cw: c.d.ClickHouse.NewWorker(i, bf),
	}
	return w.processIncomingFlow, w.shutdown
}

// shutdown shutdowns the worker, flushing any remaining data.
func (w *worker) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	w.cw.Flush(ctx)
}

// processIncomingFlow processes one incoming flow from Kafka.
func (w *worker) processIncomingFlow(ctx context.Context, data []byte) error {
	// Do nothing if we are shutting down
	if !w.c.t.Alive() {
		return kafka.ErrStopProcessing
	}

	// Raw flaw decoding: fatal
	w.c.metrics.rawFlowsReceived.Inc()
	w.rawFlow.ResetVT()
	if err := w.rawFlow.UnmarshalVT(data); err != nil {
		w.c.metrics.rawFlowsErrors.WithLabelValues("cannot decode protobuf")
		return fmt.Errorf("cannot decode raw flow: %w", err)
	}

	// Porcess each decoded flow
	finalize := func() {
		// Accounting
		exporter := w.bf.ExporterAddress.Unmap().String()
		w.c.metrics.flowsReceived.WithLabelValues(exporter).Inc()

		// Enrichment: not fatal
		ip := w.bf.ExporterAddress
		if skip := w.enrichFlow(ip, exporter); skip {
			w.bf.Undo()
			return
		}

		// If we have HTTP clients, send to them too
		if atomic.LoadUint32(&w.c.httpFlowClients) > 0 {
			if jsonBytes, err := json.Marshal(w.bf); err == nil {
				select {
				case w.c.httpFlowChannel <- jsonBytes: // OK
				default: // Overflow, best effort and ignore
				}
			}
		}

		// Finalize and forward to ClickHouse
		w.c.metrics.flowsForwarded.WithLabelValues(exporter).Inc()
		w.cw.FinalizeAndSend(ctx)
	}

	// Flow decoding: not fatal
	err := w.c.d.Flow.Decode(&w.rawFlow, w.bf, finalize)
	if err != nil {
		w.c.metrics.rawFlowsErrors.WithLabelValues("cannot decode payload")
		return nil
	}

	return nil

}
