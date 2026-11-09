// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkainput

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/reporter"
)

// ErrStopProcessing should be returned as an error when we need to stop processing more flows.
var ErrStopProcessing = errors.New("stop processing further flows")

// flushTimeout bounds a flush on stop or on revoke.
const flushTimeout = 10 * time.Second

// Consumer is a franz-go consumer and should process flow messages. Offsets are
// committed only once the records are written to ClickHouse.
type Consumer struct {
	r *reporter.Reporter
	l zerolog.Logger

	metrics  metrics
	worker   int
	callback ReceiveFunc
	flush    FlushFunc

	// mu serializes the poll loop, the stop path and the partition callbacks.
	mu sync.Mutex
	// processed holds, per partition, the offset following the last record
	// given to the callback.
	processed map[string]map[int32]kgo.EpochOffset
	stopped   bool
}

// Flushed tells what a ReceiveFunc call wrote to ClickHouse.
type Flushed int

const (
	// FlushedNothing means no batch was written.
	FlushedNothing Flushed = iota
	// FlushedBefore means previous messages were written, but not all of
	// the current one.
	FlushedBefore
	// FlushedAll means previous messages and the current one were written.
	FlushedAll
)

// ReceiveFunc is a function that will be called with each received messages.
type ReceiveFunc func(context.Context, []byte) (Flushed, error)

// FlushFunc writes the pending batch. It returns an error if it was not
// written.
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

		worker:    worker,
		metrics:   c.metrics,
		callback:  callback,
		flush:     flush,
		processed: map[string]map[int32]kgo.EpochOffset{},
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
				for _, record := range partition.Records {
					if err := ctx.Err(); err != nil {
						return err
					}
					messagesReceived.Inc()
					bytesReceived.Add(float64(len(record.Value)))
					flushed, err := c.callback(ctx, record.Value)
					if flushed == FlushedBefore {
						// Partly written: the record replays.
						c.setProcessed(record, record.Offset)
						c.commit(ctx, client)
					}
					// On error, the record is skipped: nothing of it is pending.
					c.setProcessed(record, record.Offset+1)
					if flushed == FlushedAll {
						c.commit(ctx, client)
					}
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// setProcessed records the offset to commit for the partition of a record.
func (c *Consumer) setProcessed(record *kgo.Record, offset int64) {
	partitions, ok := c.processed[record.Topic]
	if !ok {
		partitions = map[int32]kgo.EpochOffset{}
		c.processed[record.Topic] = partitions
	}
	partitions[record.Partition] = kgo.EpochOffset{Epoch: record.LeaderEpoch, Offset: offset}
}

// commit synchronously commits the processed offsets. It should be called
// with the lock held, after a successful flush.
func (c *Consumer) commit(ctx context.Context, client *kgo.Client) {
	offsets := make(map[string]map[int32]kgo.EpochOffset, len(c.processed))
	for topic, partitions := range c.processed {
		offsets[topic] = make(map[int32]kgo.EpochOffset, len(partitions))
		for partition, offset := range partitions {
			offsets[topic][partition] = offset
		}
	}
	client.CommitOffsetsSync(ctx, offsets,
		func(_ *kgo.Client, _ *kmsg.OffsetCommitRequest, resp *kmsg.OffsetCommitResponse, err error) {
			if err == nil {
				for _, topic := range resp.Topics {
					for _, partition := range topic.Partitions {
						if err = kerr.ErrorForCode(partition.ErrorCode); err != nil {
							break
						}
					}
				}
			}
			if err != nil {
				c.l.Err(err).Msg("cannot commit offsets")
			}
		})
}

// flushAndCommit flushes the pending batch and commits on success.
func (c *Consumer) flushAndCommit(ctx context.Context, client *kgo.Client) {
	ctx, cancel := context.WithTimeout(ctx, flushTimeout)
	defer cancel()
	if err := c.flush(ctx); err != nil {
		c.l.Err(err).Msg("cannot flush batch, offsets not committed")
		return
	}
	c.commit(ctx, client)
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

// forget drops the processed offsets of the given partitions.
func (c *Consumer) forget(partitions map[string][]int32) {
	for topic, ps := range partitions {
		for _, p := range ps {
			delete(c.processed[topic], p)
		}
		if len(c.processed[topic]) == 0 {
			delete(c.processed, topic)
		}
	}
}
