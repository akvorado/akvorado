// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles flow exports to Kafka.
package kafka

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/IBM/sarama"
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

	kafkaConfig         *sarama.Config
	kafkaTopic          string
	kafkaProducer       sarama.AsyncProducer
	createKafkaProducer func() (sarama.AsyncProducer, error)
	metrics             metrics
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new Kafka exporter component.
func New(reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	// Build Kafka configuration
	kafkaConfig, err := kafka.NewConfig(configuration.Configuration)
	if err != nil {
		return nil, err
	}
	kafkaConfig.Metadata.AllowAutoTopicCreation = true
	kafkaConfig.Producer.MaxMessageBytes = configuration.MaxMessageBytes
	kafkaConfig.Producer.Compression = sarama.CompressionCodec(configuration.CompressionCodec)
	kafkaConfig.Producer.Return.Successes = false
	kafkaConfig.Producer.Return.Errors = true
	kafkaConfig.Producer.Flush.Bytes = configuration.FlushBytes
	kafkaConfig.Producer.Flush.Frequency = configuration.FlushInterval
	kafkaConfig.Producer.Partitioner = sarama.NewHashPartitioner
	kafkaConfig.ChannelBufferSize = configuration.QueueSize
	if err := kafkaConfig.Validate(); err != nil {
		return nil, fmt.Errorf("cannot validate Kafka configuration: %w", err)
	}

	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: configuration,

		kafkaConfig: kafkaConfig,
		kafkaTopic:  fmt.Sprintf("%s-v%d", configuration.Topic, pb.Version),
	}
	c.initMetrics()
	c.createKafkaProducer = func() (sarama.AsyncProducer, error) {
		return sarama.NewAsyncProducer(c.config.Brokers, c.kafkaConfig)
	}
	c.d.Daemon.Track(&c.t, "inlet/kafka")
	return &c, nil
}

// Start starts the Kafka component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting Kafka component")
	kafka.GlobalKafkaLogger.Register(c.r)

	// Create producer
	kafkaProducer, err := c.createKafkaProducer()
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to create async producer")
		return fmt.Errorf("unable to create Kafka async producer: %w", err)
	}
	c.kafkaProducer = kafkaProducer

	// Main loop
	c.t.Go(func() error {
		defer kafkaProducer.Close()
		errLogger := c.r.Sample(reporter.BurstSampler(10*time.Second, 3))
		dying := c.t.Dying()
		for {
			select {
			case <-dying:
				c.r.Debug().Msg("stop error logger")
				return nil
			case msg := <-kafkaProducer.Errors():
				if msg != nil {
					c.metrics.errors.WithLabelValues(msg.Error()).Inc()
					errLogger.Err(msg.Err).
						Str("topic", msg.Msg.Topic).
						Int64("offset", msg.Msg.Offset).
						Int32("partition", msg.Msg.Partition).
						Msg("Kafka producer error")
				}
			}
		}
	})
	return nil
}

// Stop stops the Kafka component
func (c *Component) Stop() error {
	defer func() {
		c.kafkaConfig.MetricRegistry.UnregisterAll()
		kafka.GlobalKafkaLogger.Unregister()
		c.r.Info().Msg("Kafka component stopped")
	}()
	c.r.Info().Msg("stopping Kafka component")
	c.t.Kill(nil)
	return c.t.Wait()
}

// Send a message to Kafka.
func (c *Component) Send(exporter string, payload []byte) {
	c.metrics.bytesSent.WithLabelValues(exporter).Add(float64(len(payload)))
	c.metrics.messagesSent.WithLabelValues(exporter).Inc()
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, rand.Uint32())
	c.kafkaProducer.Input() <- &sarama.ProducerMessage{
		Topic: c.kafkaTopic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(payload),
	}
}
