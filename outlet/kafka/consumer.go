// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"errors"
	"strconv"

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
func (c *Consumer) ProcessFetches(ctx context.Context, client *kgo.Client, fetches kgo.Fetches) error {
	if fetches.Empty() {
		return nil
	}

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
	for _, fetch := range fetches {
		for _, topic := range fetch.Topics {
			for _, partition := range topic.Partitions {
				err := func() error {
					var epoch int32
					var offset int64
					defer func() {
						client.MarkCommitOffsets(map[string]map[int32]kgo.EpochOffset{
							topic.Topic: {
								partition.Partition: kgo.EpochOffset{Epoch: epoch, Offset: offset},
							},
						})
					}()
					for _, record := range partition.Records {
						epoch = record.LeaderEpoch
						offset = record.Offset + 1
						messagesReceived.Inc()
						bytesReceived.Add(float64(len(record.Value)))
						if err := c.callback(ctx, record.Value); err != nil {
							return err
						}
					}
					return nil
				}()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
