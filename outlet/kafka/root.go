// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles flow imports from Kafka.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IBM/sarama"
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

	kafkaConfig *sarama.Config
	kafkaTopic  string

	healthy chan reporter.ChannelHealthcheckFunc
	clients []sarama.Client
	metrics metrics
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new Kafka exporter component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (Component, error) {
	// Build Kafka configuration
	kafkaConfig, err := kafka.NewConfig(configuration.Configuration)
	if err != nil {
		return nil, err
	}
	kafkaConfig.Consumer.Fetch.Max = configuration.MaxMessageBytes
	kafkaConfig.Consumer.Fetch.Min = configuration.FetchMinBytes
	kafkaConfig.Consumer.MaxWaitTime = configuration.FetchMaxWaitTime
	kafkaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	// kafkaConfig.Consumer.Offsets.AutoCommit.Enable = false
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	kafkaConfig.Metadata.RefreshFrequency = time.Minute
	kafkaConfig.Metadata.AllowAutoTopicCreation = false
	kafkaConfig.ChannelBufferSize = configuration.QueueSize
	if err := kafkaConfig.Validate(); err != nil {
		return nil, fmt.Errorf("cannot validate Kafka configuration: %w", err)
	}

	c := realComponent{
		r:      r,
		d:      &dependencies,
		config: configuration,

		healthy:     make(chan reporter.ChannelHealthcheckFunc),
		kafkaConfig: kafkaConfig,
		kafkaTopic:  fmt.Sprintf("%s-v%d", configuration.Topic, pb.Version),
	}
	c.initMetrics()
	c.r.RegisterHealthcheck("kafka", c.channelHealthcheck())
	c.d.Daemon.Track(&c.t, "outlet/kafka")
	return &c, nil
}

// Start starts the Kafka component.
func (c *realComponent) Start() error {
	c.r.Info().Msg("starting Kafka component")
	kafka.GlobalKafkaLogger.Register(c.r)
	// Start the clients
	for i := range c.config.Workers {
		logger := c.r.With().Int("worker", i).Logger()
		logger.Debug().Msg("starting")
		client, err := sarama.NewClient(c.config.Brokers, c.kafkaConfig)
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
	topics := []string{c.kafkaTopic}
	for i := range c.config.Workers {
		callback, shutdown := workerBuilder(i)
		c.t.Go(func() error {
			logger := c.r.With().
				Int("worker", i).
				Logger()
			client, err := sarama.NewConsumerGroupFromClient(c.config.ConsumerGroup, c.clients[i])
			if err != nil {
				logger.Err(err).
					Int("worker", i).
					Str("brokers", strings.Join(c.config.Brokers, ",")).
					Msg("unable to create group consumer")
				return fmt.Errorf("unable to create Kafka group consumer: %w", err)
			}
			defer client.Close()
			consumer := c.NewConsumer(i, callback)
			defer shutdown()
			for {
				if err := client.Consume(ctx, topics, consumer); err != nil {
					if errors.Is(err, sarama.ErrClosedConsumerGroup) {
						return nil
					}
					if errors.Is(err, context.Canceled) {
						return nil
					}
					logger.Err(err).
						Int("worker", i).
						Msg("cannot get message from consumer")
					return fmt.Errorf("cannot get message from consumer: %w", err)
				}
			}
		})
	}
	return nil
}

// Stop stops the Kafka component
func (c *realComponent) Stop() error {
	defer func() {
		c.kafkaConfig.MetricRegistry.UnregisterAll()
		kafka.GlobalKafkaLogger.Unregister()
		close(c.healthy)
		for _, client := range c.clients {
			client.Close()
		}
		c.r.Info().Msg("Kafka component stopped")
	}()
	c.r.Info().Msg("stopping Kafka component")
	c.t.Kill(nil)
	return c.t.Wait()
}

func (c *realComponent) channelHealthcheck() reporter.HealthcheckFunc {
	return reporter.ChannelHealthcheck(c.t.Context(nil), c.healthy)
}
