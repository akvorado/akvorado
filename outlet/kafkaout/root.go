// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafkaout exports enriched flows to a Kafka topic, in parallel with
// the ClickHouse output. It is disabled by default.
//
// Delivery is best-effort and at-most-once: records are produced asynchronously
// from a bounded queue and, if the queue is full (a slow or broken broker) or a
// produce errors, they are dropped and counted (never retried). The flow worker
// only enqueues, so a slow Kafka output never blocks the ClickHouse path.
//
// The topic is the configured name suffixed with the schema hash, so an
// incompatible schema change lands on a new topic instead of mixing wire layouts
// for consumers. The outlet does not yet manage the topic the way the
// orchestrator manages the inlet topic, so creation and retention are configured
// at deploy time. Consumers should track the schema's Protobuf definition
// (Schema.ProtobufDefinition) for the layout.
package kafkaout

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kprom"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Component represents the Kafka output.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	kafkaOpts   []kgo.Opt
	kafkaTopic  string
	kafkaClient *kgo.Client
	sendCh      chan *kgo.Record
	errLogger   reporter.Logger
	metrics     metrics
}

// Dependencies define the dependencies of the Kafka output.
type Dependencies struct {
	Daemon daemon.Component
	Schema *schema.Component
}

// New creates a new Kafka output component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	kafkaTopic := fmt.Sprintf("%s-%s", configuration.Topic, dependencies.Schema.ProtobufMessageHash())
	c := Component{
		r:          r,
		d:          &dependencies,
		config:     configuration,
		kafkaTopic: kafkaTopic,
		errLogger:  r.Sample(reporter.BurstSampler(10*time.Second, 3)),
	}
	c.initMetrics()

	// Inert when disabled, so existing deployments are unaffected.
	if !configuration.Enabled {
		return &c, nil
	}

	kafkaOpts, err := kafka.NewConfig(r, configuration.Configuration)
	if err != nil {
		return nil, err
	}
	kafkaOpts = append(kafkaOpts,
		kgo.AllowAutoTopicCreation(),
		kgo.MaxBufferedRecords(configuration.QueueSize),
		kgo.ProducerBatchCompression(kgo.Lz4Compression()),
		kgo.RecordPartitioner(kgo.UniformBytesPartitioner(64<<20, true, true, nil)),
	)
	if err := kgo.ValidateOpts(kafkaOpts...); err != nil {
		return nil, fmt.Errorf("invalid Kafka configuration: %w", err)
	}
	c.kafkaOpts = kafkaOpts
	c.d.Daemon.Track(&c.t, "outlet/kafkaout")
	return &c, nil
}

// Enabled reports whether the Kafka output is active.
func (c *Component) Enabled() bool { return c.config.Enabled }

// Start starts the Kafka output component.
func (c *Component) Start() error {
	if !c.config.Enabled {
		return nil
	}
	c.r.Info().Msg("starting Kafka output component")

	kafkaMetrics := kprom.NewMetrics("", kprom.Histograms(kprom.RequestDurationE2E, kprom.RequestThrottled))
	kafkaClient, err := kgo.NewClient(append(c.kafkaOpts, kgo.WithHooks(kafkaMetrics))...)
	if err != nil {
		return fmt.Errorf("unable to create Kafka client: %w", err)
	}
	c.r.RegisterMetricCollector(kafkaMetrics)
	c.sendCh = make(chan *kgo.Record, c.config.QueueSize)
	c.kafkaClient = kafkaClient

	// A single drain goroutine owns Produce, so kgo's block-when-full is
	// isolated from the flow workers (which only enqueue; see Send). The
	// tomb-tied context unblocks an in-flight Produce on shutdown.
	c.t.Go(func() error {
		ctx := c.t.Context(context.Background())
		for {
			select {
			case <-c.t.Dying():
				kafkaClient.Close()
				return nil
			case record := <-c.sendCh:
				payloadLen := len(record.Value)
				kafkaClient.Produce(ctx, record, func(_ *kgo.Record, err error) {
					if err != nil {
						if ke, ok := err.(*kerr.Error); ok {
							c.metrics.errors.WithLabelValues(ke.Message).Inc()
						} else {
							c.metrics.errors.WithLabelValues("unknown").Inc()
						}
						c.errLogger.Err(err).Str("topic", c.kafkaTopic).Msg("Kafka producer error")
						return
					}
					c.metrics.messagesSent.Inc()
					c.metrics.bytesSent.Add(float64(payloadLen))
				})
			}
		}
	})
	return nil
}

// Stop stops the Kafka output component.
func (c *Component) Stop() error {
	if !c.config.Enabled {
		return nil
	}
	defer c.r.Info().Msg("Kafka output component stopped")
	c.r.Info().Msg("stopping Kafka output component")
	c.t.Kill(nil)
	return c.t.Wait()
}

// Send enqueues one enriched flow record for asynchronous production to Kafka.
// Non-blocking and best-effort: if the send queue is full (a slow or broken
// broker), the record is dropped and counted, so the flow worker — and the
// ClickHouse path — are never blocked.
func (c *Component) Send(key string, payload []byte) {
	if c.kafkaClient == nil {
		return
	}
	record := &kgo.Record{Topic: c.kafkaTopic, Key: []byte(key), Value: payload}
	select {
	case c.sendCh <- record:
	default:
		c.metrics.dropped.Inc()
	}
}
