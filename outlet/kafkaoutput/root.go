// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafkaoutput exports enriched flows to a Kafka topic, in parallel with
// the ClickHouse output. It is disabled by default.
//
// Delivery is best-effort and at-most-once: records are produced asynchronously
// and, if the producer buffer is full (a slow or broken broker) or a produce
// errors, they are dropped and counted (never retried). Send never blocks, so a
// slow Kafka output never blocks the ClickHouse path.
//
// The topic is the configured name suffixed with the schema hash, so an
// incompatible schema change lands on a new topic instead of mixing wire layouts
// for consumers. The orchestrator creates and keeps this topic in sync when its
// own kafka-output block is configured. Otherwise, the outlet asks the broker to
// create it on first produce, which needs auto-creation enabled on the broker
// and uses the broker defaults for partitions and retention. Consumers should
// track the schema's Protobuf definition (Schema.ProtobufDefinition) for the
// layout.
package kafkaoutput

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kprom"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/httpserver"
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
	errLogger   reporter.Logger
	metrics     metrics
}

// Dependencies define the dependencies of the Kafka output.
type Dependencies struct {
	Daemon daemon.Component
	HTTP   *httpserver.Component
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
		kgo.ProducerBatchCompression(kgo.CompressionCodec(configuration.CompressionCodec)),
		kgo.RecordPartitioner(kgo.UniformBytesPartitioner(64<<20, true, true, nil)),
	)
	if err := kgo.ValidateOpts(kafkaOpts...); err != nil {
		return nil, fmt.Errorf("invalid Kafka configuration: %w", err)
	}
	c.kafkaOpts = kafkaOpts
	c.d.Daemon.Track(&c.t, "outlet/kafkaoutput")
	c.d.HTTP.APIRouter.GET("/api/v0/outlet/kafka-output/schema.proto", c.SchemaHTTPHandler)
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
	c.kafkaClient = kafkaClient

	// When dying, give the buffered records a chance to reach the broker, then
	// close the client.
	c.t.Go(func() error {
		<-c.t.Dying()
		if c.config.ShutdownTimeout > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), c.config.ShutdownTimeout)
			defer cancel()
			if err := kafkaClient.Flush(ctx); err != nil {
				c.r.Warn().Err(err).Msg("cannot flush the remaining records")
			}
		}
		kafkaClient.Close()
		return nil
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

// Send hands one enriched flow record to the Kafka producer. Non-blocking and
// best-effort: if the producer buffer is full (a slow or broken broker), the
// record is dropped and counted, so the flow worker — and the ClickHouse path —
// are never blocked.
func (c *Component) Send(exporter string, payload []byte) {
	if c.kafkaClient == nil {
		return
	}
	record := &kgo.Record{
		Topic: c.kafkaTopic,
		Key:   c.config.LoadBalance.RecordKey(exporter),
		Value: payload,
	}
	c.kafkaClient.TryProduce(context.Background(), record, func(_ *kgo.Record, err error) {
		if err == nil {
			c.metrics.messagesSent.Inc()
			c.metrics.bytesSent.Add(float64(len(payload)))
			return
		}
		if errors.Is(err, kgo.ErrMaxBuffered) {
			c.metrics.dropped.Inc()
			return
		}
		if ke, ok := errors.AsType[*kerr.Error](err); ok {
			c.metrics.errors.WithLabelValues(ke.Message).Inc()
		} else {
			c.metrics.errors.WithLabelValues("unknown").Inc()
		}
		c.errLogger.Err(err).Str("topic", c.kafkaTopic).Msg("Kafka producer error")
	})
}
