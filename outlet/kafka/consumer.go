// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/reporter"
)

// ErrStopProcessing should be returned as an error when we need to stop processing more flows.
var ErrStopProcessing = errors.New("stop processing further flows")

// Consumer is a franz-go consumer and should process flow messages.
type Consumer struct {
	r *reporter.Reporter
	l zerolog.Logger

	metrics  metrics
	worker   int
	callback ReceiveFunc
	mu       sync.Mutex
}

// ReceiveFunc is a function that will be called with each received messages.
type ReceiveFunc func(context.Context, []byte) error

// ShutdownFunc is a function that will be called on shutdown of the consumer.
type ShutdownFunc func()

// WorkerBuilderFunc returns a function to be called with each received messages and
// a function to be called when shutting down.
type WorkerBuilderFunc func(int) (ReceiveFunc, ShutdownFunc)

// NewConsumer creates a new consumer.
func (c *realComponent) NewConsumer(worker int, callback ReceiveFunc) *Consumer {
	return &Consumer{
		r: c.r,
		l: c.r.With().Int("worker", worker).Logger(),

		worker:   worker,
		metrics:  c.metrics,
		callback: callback,
	}
}

// ProcessFetches processes the fetched records.
func (c *Consumer) ProcessFetches(ctx context.Context, fetches kgo.Fetches) error {
	worker := strconv.Itoa(c.worker)
	c.metrics.fetchesReceived.WithLabelValues(worker).Inc()

	if errs := fetches.Errors(); len(errs) > 0 {
		for _, err := range errs {
			if errors.Is(err.Err, context.Canceled) {
				return nil
			}
			c.metrics.errorsReceived.WithLabelValues(worker).Inc()
			c.l.Err(err.Err).
				Str("topic", err.Topic).
				Int32("partition", err.Partition).
				Msg("fetch error")
		}
		// Assume the error is fatal.
		return ErrStopProcessing
	}

	messagesReceived := c.metrics.messagesReceived.WithLabelValues(worker)
	bytesReceived := c.metrics.bytesReceived.WithLabelValues(worker)
	for iter := fetches.RecordIter(); !iter.Done(); {
		record := iter.Next()
		messagesReceived.Inc()
		bytesReceived.Add(float64(len(record.Value)))
		if err := c.callback(ctx, record.Value); err != nil {
			return err
		}
	}

	return nil
}
