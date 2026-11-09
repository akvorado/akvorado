// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkainput

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/reporter"
)

// ErrStopProcessing should be returned as an error when we need to stop processing more flows.
var ErrStopProcessing = errors.New("stop processing further flows")

// flushTimeout bounds a flush on stop or on revoke.
const flushTimeout = 10 * time.Second

// Consumer is a franz-go consumer and should process flow messages. Offsets are
// committed only once the records are flushed by the worker.
type Consumer struct {
	r *reporter.Reporter
	l zerolog.Logger

	metrics  metrics
	worker   int
	callback ReceiveFunc
	flush    FlushFunc

	// mu serializes the poll loop, the stop path and the partition callbacks.
	mu sync.Mutex
	// last holds, per partition, the last record given to the callback.
	last    map[string]map[int32]*kgo.Record
	stopped bool
}

// ReceiveFunc is a function that will be called with each received messages.
// It reports whether the worker flushed its batch, including this message.
type ReceiveFunc func(context.Context, []byte) (bool, error)

// FlushFunc flushes the pending batch of the worker. It returns an error if
// the batch was not flushed.
type FlushFunc func(context.Context) error

// ShutdownFunc is a function that will be called on shutdown of the consumer.
type ShutdownFunc func()

// WorkerBuilderFunc returns a function to be called with each received
// messages, a function to flush the pending batch and a function to be called
// when shutting down. It is provided the worker number (for logging purpose)
// as well as a chan for the worker to request more or less workers.
type WorkerBuilderFunc func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc)

// NewConsumer creates a new consumer.
func (c *realComponent) newConsumer(worker int, callback ReceiveFunc, flush FlushFunc) *Consumer {
	return &Consumer{
		r: c.r,
		l: c.r.With().Int("worker", worker).Logger(),

		worker:   worker,
		metrics:  c.metrics,
		callback: callback,
		flush:    flush,
		last:     map[string]map[int32]*kgo.Record{},
	}
}

// ProcessFetches processes the fetched records.
func (c *Consumer) ProcessFetches(ctx context.Context, client *kgo.Client, fetches kgo.Fetches) error {
	if fetches.Empty() {
		return nil
	}

	worker := strconv.Itoa(c.worker)
	c.metrics.fetchesReceived.WithLabelValues(worker).Inc()

	if errs := fetches.Errors(); len(errs) > 0 {
		for _, err := range errs {
			if errors.Is(err.Err, context.Canceled) || errors.Is(err.Err, ErrStopProcessing) {
				return nil
			}
			c.metrics.errorsReceived.WithLabelValues(worker).Inc()
			c.l.Err(err.Err).
				Str("topic", err.Topic).
				Int32("partition", err.Partition).
				Msg("fetch error")
		}
		// Assume the error is fatal. The data is uncommitted, so it should be OK.
		return ErrStopProcessing
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	messagesReceived := c.metrics.messagesReceived.WithLabelValues(worker)
	bytesReceived := c.metrics.bytesReceived.WithLabelValues(worker)
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				if err := c.processPartition(ctx, client, partition.Records, messagesReceived, bytesReceived); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// processPartition gives the records of one partition to the callback. It
// commits after each flush.
func (c *Consumer) processPartition(ctx context.Context, client *kgo.Client, records []*kgo.Record, messagesReceived, bytesReceived prometheus.Counter) error {
	var last *kgo.Record
	defer func() {
		if last != nil {
			c.setLast(last)
		}
	}()
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		messagesReceived.Inc()
		bytesReceived.Add(float64(len(record.Value)))
		flushed, err := c.callback(ctx, record.Value)
		// On error, the record is skipped: nothing of it is pending.
		last = record
		if flushed {
			c.setLast(record)
			if err := c.commit(ctx, client); err != nil {
				c.l.Err(err).Msg("cannot commit offsets")
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// setLast records the last record given to the callback for its partition.
func (c *Consumer) setLast(record *kgo.Record) {
	partitions, ok := c.last[record.Topic]
	if !ok {
		partitions = map[int32]*kgo.Record{}
		c.last[record.Topic] = partitions
	}
	partitions[record.Partition] = record
}

// commit synchronously commits the records given to the callback. It should be
// called with the lock held, after a successful flush.
func (c *Consumer) commit(ctx context.Context, client *kgo.Client) error {
	records := []*kgo.Record{}
	for _, partitions := range c.last {
		for _, record := range partitions {
			records = append(records, record)
		}
	}
	if len(records) == 0 {
		return nil
	}
	return client.CommitRecords(ctx, records...)
}

// flushAndCommit flushes the pending batch and commits on success.
func (c *Consumer) flushAndCommit(ctx context.Context, client *kgo.Client) {
	ctx, cancel := context.WithTimeout(ctx, flushTimeout)
	defer cancel()
	if err := c.flush(ctx); err != nil {
		c.l.Err(err).Msg("cannot flush batch, offsets not committed")
		return
	}
	if err := c.commit(ctx, client); err != nil {
		c.l.Err(err).Msg("cannot commit offsets")
	}
}

// stop flushes and commits before the client leaves the group. Partition
// callbacks are no-ops after this.
func (c *Consumer) stop(client *kgo.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushAndCommit(context.Background(), client)
	c.stopped = true
}

// onPartitionsRevoked flushes and commits before partitions leave. With
// BlockRebalanceOnPoll, it does not run during ProcessFetches.
func (c *Consumer) onPartitionsRevoked(ctx context.Context, client *kgo.Client, revoked map[string][]int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || len(revoked) == 0 {
		return
	}
	c.flushAndCommit(ctx, client)
	c.forget(revoked)
}

// onPartitionsLost forgets lost partitions: committing them would fail.
func (c *Consumer) onPartitionsLost(_ context.Context, _ *kgo.Client, lost map[string][]int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forget(lost)
}

// forget drops the last records of the given partitions.
func (c *Consumer) forget(partitions map[string][]int32) {
	for topic, ps := range partitions {
		for _, p := range ps {
			delete(c.last[topic], p)
		}
		if len(c.last[topic]) == 0 {
			delete(c.last, topic)
		}
	}
}
