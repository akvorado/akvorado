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

	scaleRequestChan chan<- kafka.ScaleRequest
}

// newWorker instantiates a new worker and returns a callback function to
// process an incoming flow and a function to call on shutdown.
func (c *Component) newWorker(i int, scaleRequestChan chan<- kafka.ScaleRequest) (kafka.ReceiveFunc, kafka.ShutdownFunc) {
	bf := c.d.Schema.NewFlowMessage()
	w := worker{
		c:                c,
		l:                c.r.With().Int("worker", i).Logger(),
		bf:               bf,
		cw:               c.d.ClickHouse.NewWorker(i, bf),
		scaleRequestChan: scaleRequestChan,
	}
	return w.processIncomingFlow, w.shutdown
}

// shutdown shutdowns the worker, flushing any remaining data.
func (w *worker) shutdown() {
	w.l.Info().Msg("flush final batch to ClickHouse")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w.cw.Flush(ctx)
	w.l.Info().Msg("worker stopped")
}

// processIncomingFlow processes one incoming flow from Kafka.
func (w *worker) processIncomingFlow(ctx context.Context, data []byte) error {
	// Raw flow decoding
	w.c.metrics.rawFlowsReceived.Inc()
	w.rawFlow.ResetVT()
	if err := w.rawFlow.UnmarshalVT(data); err != nil {
		w.c.metrics.rawFlowsErrors.WithLabelValues("cannot decode protobuf")
		return fmt.Errorf("cannot decode raw flow: %w", err)
	}

	// Process each decoded flow
	rateLimit := w.rawFlow.RateLimit
	finalize := func() {
		// Accounting
		exporter := w.bf.ExporterAddress.Unmap().String()
		w.c.metrics.flowsReceived.WithLabelValues(exporter).Inc()

		// Rate limiting
		ip := w.bf.ExporterAddress
		if rateLimit > 0 {
			allowed, dropRate := w.c.rateLimiter.allowOneMessage(ip, rateLimit)
			if !allowed {
				w.c.metrics.flowsRateLimited.WithLabelValues(exporter).Inc()
				w.bf.Undo()
				return
			}
			if dropRate > 0 {
				w.bf.SamplingRate = uint64(float64(w.bf.SamplingRate) / (1 - dropRate))
			}
		}

		// Enrichment
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
		status := w.cw.FinalizeAndSend(ctx)
		switch status {
		case clickhouse.WorkerStatusOverloaded:
			w.scaleRequestChan <- kafka.ScaleIncrease
		case clickhouse.WorkerStatusUnderloaded:
			w.scaleRequestChan <- kafka.ScaleDecrease
		case clickhouse.WorkerStatusSteady:
			w.scaleRequestChan <- kafka.ScaleSteady
		}
	}

	// Flow decoding
	err := w.c.d.Flow.Decode(&w.rawFlow, w.bf, finalize)
	if err != nil {
		// w.bf.ExporterAddress may not be known yet, so increase raw_flows_errors_total.
		w.c.metrics.rawFlowsErrors.WithLabelValues("cannot decode payload").Inc()
		return nil
	}

	return nil
}
