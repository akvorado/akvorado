// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles flow imports from Kafka.
package kafka

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
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
	StopWorkers()
	Stop() error
}

// realComponent implements the Kafka consumer.
type realComponent struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	kafkaOpts    []kgo.Opt
	kadmClient   *kadm.Client
	kadmClientMu sync.Mutex
	kafkaMetrics []*kprom.Metrics

	workerMu          sync.Mutex
	workers           []worker
	workerBuilder     WorkerBuilderFunc
	workerRequestChan chan<- ScaleRequest
	metrics           metrics
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

		kafkaMetrics: []*kprom.Metrics{},
	}
	c.initMetrics()

	kafkaOpts = append(kafkaOpts,
		kgo.FetchMinBytes(configuration.FetchMinBytes),
		kgo.FetchMaxWait(configuration.FetchMaxWaitTime),
		kgo.ConsumerGroup(configuration.ConsumerGroup),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtEnd()),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.ConsumeTopics(fmt.Sprintf("%s-v%d", configuration.Topic, pb.Version)),
		kgo.AutoCommitMarks(),
		kgo.AutoCommitInterval(time.Second),
		kgo.OnPartitionsRevoked(c.onPartitionsRevoked),
		kgo.BlockRebalanceOnPoll(),
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

	// Create an admin Kafka client
	kafkaOpts, err := kafka.NewConfig(c.r, c.config.Configuration)
	if err != nil {
		return err
	}

	kadmClient, err := kadm.NewOptClient(kafkaOpts...)
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to create Kafka admin client")
		return fmt.Errorf("unable to create Kafka admin client: %w", err)
	}

	// Check the number of partitions
	topics, err := kadmClient.ListTopics(context.Background())
	if err != nil {
		return fmt.Errorf("unable to get metadata for topics: %w", err)
	}
	topicName := fmt.Sprintf("%s-v%d", c.config.Topic, pb.Version)
	topic, ok := topics[topicName]
	if !ok {
		return fmt.Errorf("unable find topic %q", topicName)
	}
	nbPartitions := len(topic.Partitions)
	c.r.Info().Msgf("topic %q has %d partitions", topicName, nbPartitions)
	if nbPartitions < c.config.MaxWorkers {
		c.r.Warn().Msgf("capping max workers from %d to %d", c.config.MaxWorkers, nbPartitions)
		c.workerMu.Lock()
		c.config.MaxWorkers = nbPartitions
		c.workerMu.Unlock()
	}

	c.kadmClientMu.Lock()
	defer c.kadmClientMu.Unlock()
	c.kadmClient = kadmClient

	return nil
}

// StartWorkers will start the initial workers. This should only be called once.
func (c *realComponent) StartWorkers(workerBuilder WorkerBuilderFunc) error {
	c.workerRequestChan = runScaler(c.t.Context(nil), scalerConfiguration{
		minWorkers:        c.config.MinWorkers,
		maxWorkers:        c.config.MaxWorkers,
		increaseRateLimit: c.config.WorkerIncreaseRateLimit,
		decreaseRateLimit: c.config.WorkerDecreaseRateLimit,
		getWorkerCount: func() int {
			c.workerMu.Lock()
			defer c.workerMu.Unlock()
			return len(c.workers)
		},
		increaseWorkers: func(from, to int) {
			c.r.Info().Msgf("increase number of workers from %d to %d", from, to)
			for i := from; i < to; i++ {
				if err := c.startOneWorker(); err != nil {
					c.r.Err(err).Msg("cannot spawn a new worker")
					return
				}
			}
		},
		decreaseWorkers: func(from, to int) {
			c.r.Info().Msgf("decrease number of workers from %d to %d", from, to)
			for i := from; i > to; i-- {
				c.stopOneWorker()
			}
		},
	})
	c.workerBuilder = workerBuilder
	for range c.config.MinWorkers {
		if err := c.startOneWorker(); err != nil {
			return err
		}
	}
	return nil
}

// StopWorkers stops all workers
func (c *realComponent) StopWorkers() {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	for _, worker := range c.workers {
		worker.stop()
	}
}

// Stop stops the Kafka component
func (c *realComponent) Stop() error {
	defer func() {
		c.StopWorkers()
		c.kadmClientMu.Lock()
		defer c.kadmClientMu.Unlock()
		if c.kadmClient != nil {
			c.kadmClient.Close()
			c.kadmClient = nil
		}
		c.r.Info().Msg("Kafka component stopped")
	}()
	c.r.Info().Msg("stopping Kafka component")
	c.t.Kill(nil)
	return c.t.Wait()
}
