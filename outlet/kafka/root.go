// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles flow imports from Kafka.
package kafka

import (
	"fmt"
	"sync"
	"time"

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

	kafkaOpts    []kgo.Opt
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
	return nil
}

// StartWorkers will start the initial workers. This should only be called once.
func (c *realComponent) StartWorkers(workerBuilder WorkerBuilderFunc) error {
	c.workerRequestChan = c.startScaler()
	c.workerBuilder = workerBuilder
	for range c.config.MinWorkers {
		if err := c.startOneWorker(); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the Kafka component
func (c *realComponent) Stop() error {
	defer func() {
		c.stopAllWorkers()
		c.r.Info().Msg("Kafka component stopped")
	}()
	c.r.Info().Msg("stopping Kafka component")
	c.t.Kill(nil)
	return c.t.Wait()
}
