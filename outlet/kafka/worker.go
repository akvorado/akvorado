// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kprom"
)

// worker represents a worker
type worker struct {
	stop func()
}

// newClient returns a new Kafka client
func (c *realComponent) newClient(i int) (*kgo.Client, error) {
	logger := c.r.With().Int("worker", i).Logger()
	logger.Info().Msg("starting new client")
	kafkaMetrics := kprom.NewMetrics("", kprom.WithStaticLabel(prometheus.Labels{"worker": strconv.Itoa(i)}))
	kafkaOpts := append(c.kafkaOpts, kgo.WithHooks(kafkaMetrics))
	client, err := kgo.NewClient(kafkaOpts...)
	if err != nil {
		logger.Err(err).
			Int("worker", i).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to create new client")
		return nil, fmt.Errorf("unable to create Kafka client: %w", err)
	}
	c.r.RegisterMetricCollector(kafkaMetrics)
	return client, nil
}

// startOneWorker starts a new worker.
func (c *realComponent) startOneWorker() error {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()

	// New consumer
	i := len(c.workers)
	if i > c.config.MaxWorkers {
		c.r.Info().Int("Workers", c.config.MaxWorkers).Msg("maximum number of worker reached")
		return nil
	}
	client, err := c.newClient(i)
	if err != nil {
		return err
	}
	callback, shutdown := c.workerBuilder(i, c.workerRequestChan)
	consumer := c.NewConsumer(i, callback)

	// Goroutine for worker
	ctx, cancel := context.WithCancelCause(context.Background())
	ctx = c.t.Context(ctx)
	c.t.Go(func() error {
		logger := c.r.With().
			Int("worker", i).
			Logger()
		defer func() {
			logger.Info().Msg("stopping worker")

			// Allow a small grace time to commit uncommited work.
			ctx, cancelCommit := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelCommit()
			if err := client.CommitMarkedOffsets(ctx); err != nil {
				logger.Err(err).Msg("cannot commit marked partition offsets")
			}

			shutdown()
			client.CloseAllowingRebalance()
		}()

		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				fetches := client.PollFetches(ctx)
				if fetches.IsClientClosed() {
					logger.Error().Msg("client is closed")
					return errors.New("client is closed")
				}
				if err := consumer.ProcessFetches(ctx, client, fetches); err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, ErrStopProcessing) {
						return nil
					}
					logger.Err(err).Msg("cannot process fetched messages")
					return fmt.Errorf("cannot process fetched messages: %w", err)
				}
				client.AllowRebalance()
			}
		}
	})

	c.workers = append(c.workers, worker{
		stop: func() {
			cancel(ErrStopProcessing)
		},
	})
	c.metrics.workerIncrease.Inc()
	return nil
}

// stopOneWorker stops the last worker
func (c *realComponent) stopOneWorker() {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	i := len(c.workers) - 1
	if i < c.config.MinWorkers {
		c.r.Info().Int("Workers", c.config.MinWorkers).Msg("minimum number of workers reached")
		return
	}
	worker := c.workers[i]
	worker.stop()
	c.workers = c.workers[:i]
	c.metrics.workerDecrease.Inc()
}

// stopAllWorkers stops all workers
func (c *realComponent) stopAllWorkers() {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	for _, worker := range c.workers {
		worker.stop()
	}
}

// onPartitionsRevoked is called when partitions are revoked. We need to commit.
func (c *realComponent) onPartitionsRevoked(ctx context.Context, client *kgo.Client, _ map[string][]int32) {
	if err := client.CommitMarkedOffsets(ctx); err != nil {
		c.r.Err(err).Msg("cannot commit marked offsets on partition revoked")
	}
}
