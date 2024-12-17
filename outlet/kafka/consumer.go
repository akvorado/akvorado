// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog"

	"akvorado/common/reporter"
)

// ErrStopProcessing should be returned as an error when we need to stop processing more flows.
var ErrStopProcessing = errors.New("stop processing further flows")

// Consumer is a Sarama consumer group consumer and should process flow
// messages.
type Consumer struct {
	r *reporter.Reporter
	l zerolog.Logger

	healthy  chan reporter.ChannelHealthcheckFunc
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

		healthy:  c.healthy,
		worker:   worker,
		metrics:  c.metrics,
		callback: callback,
	}
}

// Setup is called at the beginning of a new consumer session, before
// ConsumeClaim.
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	c.l.Debug().Msg("start consumer group")
	return nil
}

// Cleanup is called once all ConsumeClaim goroutines have exited
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	c.l.Debug().Msg("stop consumer group")
	return nil
}

// ConsumeClaim should process the incoming claims.
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	l := c.l.With().
		Str("topic", claim.Topic()).
		Int32("partition", claim.Partition()).
		Int64("offset", claim.InitialOffset()).Logger()
	l.Debug().Msg("process new consumer group claim")
	worker := strconv.Itoa(c.worker)
	c.metrics.claimsReceived.WithLabelValues(worker).Inc()
	messagesReceived := c.metrics.messagesReceived.WithLabelValues(worker)
	bytesReceived := c.metrics.bytesReceived.WithLabelValues(worker)
	ctx := session.Context()

	for {
		select {
		case cb, ok := <-c.healthy:
			if ok {
				cb(reporter.HealthcheckOK, fmt.Sprintf("worker %d ok", c.worker))
			}
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			messagesReceived.Inc()
			bytesReceived.Add(float64(len(message.Value)))

			// ConsumeClaim can be called from multiple goroutines. We want each
			// worker/consumer to not invoke callbacks concurrently.
			c.mu.Lock()
			if err := c.callback(ctx, message.Value); err == ErrStopProcessing {
				c.mu.Unlock()
				return nil
			} else if err != nil {
				c.mu.Unlock()
				c.metrics.errorsReceived.WithLabelValues(worker).Inc()
				l.Err(err).Msg("unable to handle incoming message")
				return fmt.Errorf("unable to handle incoming message: %w", err)
			}
			c.mu.Unlock()
			session.MarkMessage(message, "")
		case <-ctx.Done():
			return nil
		}
	}
}
