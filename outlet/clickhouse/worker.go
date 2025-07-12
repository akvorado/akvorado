// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/cenkalti/backoff/v4"

	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Worker represents a worker sending to ClickHouse. It is synchronous (no
// goroutines) and most functions are bound to a context.
type Worker interface {
	FinalizeAndSend(context.Context)
	Flush(context.Context)
}

// realWorker is a working implementation of Worker.
type realWorker struct {
	c      *realComponent
	bf     *schema.FlowMessage
	last   time.Time
	logger reporter.Logger

	conn    *ch.Client
	servers []string
	options ch.Options
}

// NewWorker creates a new worker to push data to ClickHouse.
func (c *realComponent) NewWorker(i int, bf *schema.FlowMessage) Worker {
	opts, servers := c.d.ClickHouse.ChGoOptions()
	w := realWorker{
		c:      c,
		bf:     bf,
		logger: c.r.With().Int("worker", i).Logger(),

		servers: servers,
		options: opts,
	}
	return &w
}

// FinalizeAndSend sends data to ClickHouse after finalizing if we have a full batch or exceeded the maximum wait time.
func (w *realWorker) FinalizeAndSend(ctx context.Context) {
	w.bf.Finalize()
	now := time.Now()
	if w.bf.FlowCount() >= int(w.c.config.MaximumBatchSize) || w.last.Add(w.c.config.MaximumWaitTime).Before(now) {
		// Record wait time since last send
		if !w.last.IsZero() {
			waitTime := now.Sub(w.last)
			w.c.metrics.waitTime.Observe(waitTime.Seconds())
		}
		w.Flush(ctx)
		w.last = now
	}
}

// Flush sends remaining data to ClickHouse without an additional condition. It
// should be called before shutting down to flush remaining data. Otherwise,
// FinalizeAndSend() should be used instead.
func (w *realWorker) Flush(ctx context.Context) {
	// We try to send as long as possible. The only exit condition is an
	// expiration of the context.
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 0
	b.MaxInterval = 30 * time.Second
	b.InitialInterval = 20 * time.Millisecond
	backoff.Retry(func() error {
		// Connect or reconnect if connection is broken.
		if err := w.connect(ctx); err != nil {
			w.logger.Err(err).Msg("cannot connect to ClickHouse")
			return err
		}

		// Send to ClickHouse in flows_XXXXX_raw.
		start := time.Now()
		if err := w.conn.Do(ctx, ch.Query{
			Body:  w.bf.ClickHouseProtoInput().Into(fmt.Sprintf("flows_%s_raw", w.c.d.Schema.ClickHouseHash())),
			Input: w.bf.ClickHouseProtoInput(),
		}); err != nil {
			w.logger.Err(err).Msg("cannot send batch to ClickHouse")
			w.c.metrics.errors.WithLabelValues("send").Inc()
			return err
		}
		pushDuration := time.Since(start)
		w.c.metrics.insertTime.Observe(pushDuration.Seconds())
		w.c.metrics.batches.Inc()
		w.c.metrics.flows.Add(float64(w.bf.FlowCount()))

		// Clear batch
		w.bf.Clear()
		return nil
	}, backoff.WithContext(b, ctx))
}

// connect establishes or reestablish the connection to ClickHouse.
func (w *realWorker) connect(ctx context.Context) error {
	// If connection exists and is healthy, reuse it
	if w.conn != nil {
		if err := w.conn.Ping(ctx); err == nil {
			return nil
		}
		// Connection is unhealthy, close it
		w.conn.Close()
		w.conn = nil
	}

	// Try each server until one connects successfully
	var lastErr error
	for _, idx := range rand.Perm(len(w.servers)) {
		w.options.Address = w.servers[idx]
		conn, err := ch.Dial(ctx, w.options)
		if err != nil {
			w.logger.Err(err).Str("server", w.options.Address).Msg("failed to connect to ClickHouse server")
			w.c.metrics.errors.WithLabelValues("connect").Inc()
			lastErr = err
			continue
		}

		// Test the connection
		if err := conn.Ping(ctx); err != nil {
			w.logger.Err(err).Str("server", w.options.Address).Msg("ClickHouse server ping failed")
			w.c.metrics.errors.WithLabelValues("ping").Inc()
			conn.Close()
			conn = nil
			lastErr = err
			continue
		}

		// Success
		w.conn = conn
		w.logger.Info().Str("server", w.options.Address).Msg("connected to ClickHouse server")
		return nil
	}

	return lastErr
}
