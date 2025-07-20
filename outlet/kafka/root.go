// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles flow imports from Kafka.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kprom"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

// Component is the interface a Kafka consumer should implement.
type Component interface {
	StartWorkers(WorkerBuilderFunc) error
	Stop() error
}

// realComponent implements the Kafka consumer.
type realComponent struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	kafkaOpts []kgo.Opt

	clients []*kgo.Client
	metrics metrics
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new Kafka exporter component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (Component, error) {
	// Build Kafka configuration
	kafkaOpts, err := kafka.NewConfig(r, configuration.Configuration)
	if err != nil {
		return nil, err
	}

	c := realComponent{
		r:      r,
		d:      &dependencies,
		config: configuration,
	}
	c.initMetrics()

	kafkaOpts = append(kafkaOpts,
		kgo.FetchMinBytes(configuration.FetchMinBytes),
		kgo.FetchMaxWait(configuration.FetchMaxWaitTime),
		kgo.ConsumerGroup(configuration.ConsumerGroup),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtEnd()),
		kgo.ConsumeTopics(fmt.Sprintf("%s-v%d", configuration.Topic, pb.Version)),
		// Do not use kgo.BlockRebalanceOnPoll(). It needs more code to ensure
		// we are not blocked while polling.
	)

	if err := kgo.ValidateOpts(kafkaOpts...); err != nil {
		return nil, fmt.Errorf("invalid Kafka configuration: %w", err)
	}
	c.kafkaOpts = kafkaOpts
	c.d.Daemon.Track(&c.t, "outlet/kafka")
	return &c, nil
}

// Start starts the Kafka component.
func (c *realComponent) Start() error {
	c.r.Info().Msg("starting Kafka component")
	// Start the clients
	for i := range c.config.Workers {
		logger := c.r.With().Int("worker", i).Logger()
		logger.Debug().Msg("starting")

		kafkaMetrics := kprom.NewMetrics("", kprom.WithStaticLabel(prometheus.Labels{"worker": strconv.Itoa(i)}))
		kafkaOpts := append(c.kafkaOpts, kgo.WithHooks(kafkaMetrics))
		c.r.MetricCollectorForCurrentModule(kafkaMetrics)
		client, err := kgo.NewClient(kafkaOpts...)
		if err != nil {
			logger.Err(err).
				Int("worker", i).
				Str("brokers", strings.Join(c.config.Brokers, ",")).
				Msg("unable to create new client")
			return fmt.Errorf("unable to create Kafka client: %w", err)
		}
		c.clients = append(c.clients, client)
	}
	return nil
}

// StartWorkers will start the workers. This should only be called once.
func (c *realComponent) StartWorkers(workerBuilder WorkerBuilderFunc) error {
	ctx := c.t.Context(context.Background())
	for i := range c.config.Workers {
		callback, shutdown := workerBuilder(i)
		consumer := c.NewConsumer(i, callback)
		client := c.clients[i]
		c.t.Go(func() error {
			logger := c.r.With().
				Int("worker", i).
				Logger()
			defer shutdown()

			for {
				select {
				case <-ctx.Done():
					return nil
				default:
					fetches := client.PollFetches(ctx)
					if err := consumer.ProcessFetches(ctx, fetches); err != nil {
						if errors.Is(err, context.Canceled) || errors.Is(err, ErrStopProcessing) {
							return nil
						}
						logger.Err(err).
							Int("worker", i).
							Msg("cannot process fetched messages")
						return fmt.Errorf("cannot process fetched messages: %w", err)
					}
				}
			}
		})
	}
	return nil
}

// Stop stops the Kafka component
func (c *realComponent) Stop() error {
	defer func() {
		for _, client := range c.clients {
			client.CloseAllowingRebalance()
		}
		c.r.Info().Msg("Kafka component stopped")
	}()
	c.r.Info().Msg("stopping Kafka component")
	c.t.Kill(nil)
	return c.t.Wait()
}
