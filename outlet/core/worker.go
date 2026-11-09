// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
	"akvorado/outlet/kafkainput"
)

// worker represents a worker processing incoming flows.
type worker struct {
	c       *Component
	l       reporter.Logger
	cw      clickhouse.Worker
	bf      *schema.FlowMessage
	rawFlow pb.RawFlow

	scaleRequestChan chan<- kafkainput.ScaleRequest
}

// newWorker instantiates a new worker and returns a callback function to
// process an incoming flow, a function to flush the pending batch and a
// function to call on shutdown.
func (c *Component) newWorker(i int, scaleRequestChan chan<- kafkainput.ScaleRequest) (kafkainput.ReceiveFunc, kafkainput.FlushFunc, kafkainput.ShutdownFunc) {
	bf := c.d.Schema.NewFlowMessage()
	// Encode enriched flows to Protobuf in parallel with the ClickHouse batch
	// only when the Kafka output is enabled, so the ClickHouse-only path is
	// unaffected.
	if c.d.KafkaOutput != nil && c.d.KafkaOutput.Enabled() {
		bf.EnableProtobuf()
	}
	w := worker{
		c:                c,
		l:                c.r.With().Int("worker", i).Logger(),
		bf:               bf,
		cw:               c.d.ClickHouse.NewWorker(i, bf),
		scaleRequestChan: scaleRequestChan,
	}
	return w.processIncomingFlow, w.cw.Flush, w.shutdown
}

// shutdown releases the ClickHouse connections of the worker. The final batch
// is flushed before, by the Kafka consumer.
func (w *worker) shutdown() {
	w.cw.Close()
	w.l.Info().Msg("worker stopped")
}

// processIncomingFlow processes one incoming flow from Kafka.
func (w *worker) processIncomingFlow(ctx context.Context, data []byte) (kafkainput.Flushed, error) {
	// Raw flow decoding
	w.c.metrics.rawFlowsReceived.Inc()
	w.rawFlow.ResetVT()
	if err := w.rawFlow.UnmarshalVT(data); err != nil {
		w.c.metrics.rawFlowsErrors.WithLabelValues("cannot decode protobuf")
		return kafkainput.FlushedNothing, fmt.Errorf("cannot decode raw flow: %w", err)
	}

	// Process each decoded flow
	rateLimit := w.rawFlow.RateLimit
	timeReceived := w.rawFlow.TimeReceived
	flushed := false
	finalize := func() {
		// Accounting
		exporter := w.bf.ExporterAddress.Unmap().String()
		w.c.metrics.flowsReceived.WithLabelValues(exporter).Inc()

		// Rate limiting
		ip := w.bf.ExporterAddress
		samplingRateFactor := float64(1)
		if rateLimit > 0 {
			var allowed bool
			allowed, samplingRateFactor = w.c.rateLimiter.allowOneMessage(ip, rateLimit, timeReceived)
			if !allowed {
				w.c.metrics.flowsRateLimited.WithLabelValues(exporter).Inc()
				w.bf.Undo()
				return
			}
		}

		// Enrichment
		if skip := w.enrichFlow(ip, exporter); skip {
			w.bf.Undo()
			return
		}

		// Update sampling rate to account for rate limiting
		if samplingRateFactor > 1 {
			w.bf.SamplingRate = uint64(float64(w.bf.SamplingRate) * samplingRateFactor)
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
		status, ok := w.cw.FinalizeAndSend(ctx)
		flushed = flushed || ok
		// Export the enriched flow to Kafka, in parallel with ClickHouse, if
		// enabled. FinalizeAndSend has finalized the flow, so the Protobuf
		// message reflects the full enriched flow (including the fixed fields
		// appended during Finalize). Best-effort: a slow/broken Kafka output
		// must not stall the ClickHouse path.
		if w.c.d.KafkaOutput != nil && w.c.d.KafkaOutput.Enabled() {
			if payload := w.bf.ProtobufMessage(); len(payload) > 0 {
				w.c.d.KafkaOutput.Send(exporter, payload)
			}
		}
		switch status {
		case clickhouse.WorkerStatusOverloaded:
			w.scaleRequestChan <- kafkainput.ScaleIncrease
		case clickhouse.WorkerStatusUnderloaded:
			w.scaleRequestChan <- kafkainput.ScaleDecrease
		case clickhouse.WorkerStatusSteady:
			w.scaleRequestChan <- kafkainput.ScaleSteady
		}
	}

	// Flow decoding
	err := w.c.d.Flow.Decode(&w.rawFlow, w.bf, finalize)
	if err != nil {
		// w.bf.ExporterAddress may not be known yet, so increase raw_flows_errors_total.
		w.c.metrics.rawFlowsErrors.WithLabelValues("cannot decode payload").Inc()
	}

	switch {
	case !flushed:
		return kafkainput.FlushedNothing, nil
	case w.bf.FlowCount() == 0:
		return kafkainput.FlushedAll, nil
	default:
		return kafkainput.FlushedBefore, nil
	}
}
