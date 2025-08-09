// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles flow exports to Kafka.
package kafka

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kprom"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

// Component represents the Kafka exporter.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	kafkaOpts   []kgo.Opt
	kafkaTopic  string
	kafkaClient *kgo.Client
	errLogger   reporter.Logger
	metrics     metrics
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new Kafka exporter component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	// Build Kafka configuration
	kafkaOpts, err := kafka.NewConfig(r, configuration.Configuration)
	if err != nil {
		return nil, err
	}

	c := Component{
		r:          r,
		d:          &dependencies,
		config:     configuration,
		kafkaTopic: fmt.Sprintf("%s-v%d", configuration.Topic, pb.Version),
		errLogger:  r.Sample(reporter.BurstSampler(10*time.Second, 3)),
	}
	c.initMetrics()

	// Initialize options error to be able to validate them.
	kafkaOpts = append(kafkaOpts,
		kgo.AllowAutoTopicCreation(),
		kgo.MaxBufferedRecords(configuration.QueueSize),
		kgo.ProducerBatchCompression(kgo.CompressionCodec(configuration.CompressionCodec)),
		kgo.RecordPartitioner(kgo.UniformBytesPartitioner(64<<20, true, true, nil)),
	)

	if err := kgo.ValidateOpts(kafkaOpts...); err != nil {
		return nil, fmt.Errorf("invalid Kafka configuration: %w", err)
	}
	c.kafkaOpts = kafkaOpts
	c.d.Daemon.Track(&c.t, "inlet/kafka")
	return &c, nil
}

// Start starts the Kafka component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting Kafka component")

	kafkaMetrics := kprom.NewMetrics("")
	kafkaClient, err := kgo.NewClient(append(c.kafkaOpts, kgo.WithHooks(kafkaMetrics))...)
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to create Kafka client")
		return fmt.Errorf("unable to create Kafka client: %w", err)
	}
	c.r.MetricCollectorForCurrentModule(kafkaMetrics)
	c.kafkaClient = kafkaClient

	// When dying, close the client
	c.t.Go(func() error {
		<-c.t.Dying()
		kafkaClient.Close()
		return nil
	})
	return nil
}

// Stop stops the Kafka component
func (c *Component) Stop() error {
	defer c.r.Info().Msg("Kafka component stopped")
	c.r.Info().Msg("stopping Kafka component")
	c.t.Kill(nil)
	return c.t.Wait()
}

// Send a message to Kafka.
func (c *Component) Send(exporter string, payload []byte, finalizer func()) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, rand.Uint32())
	record := &kgo.Record{
		Topic: c.kafkaTopic,
		Key:   key,
		Value: payload,
	}
	c.kafkaClient.Produce(context.Background(), record, func(r *kgo.Record, err error) {
		if err == nil {
			c.metrics.bytesSent.WithLabelValues(exporter).Add(float64(len(payload)))
			c.metrics.messagesSent.WithLabelValues(exporter).Inc()
		} else {
			if ke, ok := err.(*kerr.Error); ok {
				c.metrics.errors.WithLabelValues(ke.Message).Inc()
			} else {
				c.metrics.errors.WithLabelValues("unknown").Inc()
			}
			c.errLogger.Err(err).
				Str("topic", c.kafkaTopic).
				Int64("offset", r.Offset).
				Int32("partition", r.Partition).
				Msg("Kafka producer error")
		}
		finalizer()
	})
}
