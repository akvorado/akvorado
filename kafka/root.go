// Package kafka handles flow exports to Kafka.
package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"golang.org/x/time/rate"
	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/flow"
	"akvorado/reporter"
)

// Component represents the Kafka exporter.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	kafkaTopic          string
	kafkaConfig         *sarama.Config
	kafkaProducer       sarama.AsyncProducer
	createKafkaProducer func() (sarama.AsyncProducer, error)
	metrics             metrics
}

// Dependencies define the dependencies of the Kafka exporter.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new HTTP component.
func New(reporter *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	// Build Kafka configuration
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.KafkaVersion(configuration.Version)
	kafkaConfig.Metadata.AllowAutoTopicCreation = false
	kafkaConfig.Producer.MaxMessageBytes = configuration.MaxMessageBytes
	kafkaConfig.Producer.Compression = sarama.CompressionCodec(configuration.CompressionCodec)
	kafkaConfig.Producer.Return.Successes = false
	kafkaConfig.Producer.Return.Errors = true
	kafkaConfig.Producer.Flush.Bytes = configuration.FlushBytes
	kafkaConfig.Producer.Flush.Frequency = configuration.FlushInterval
	kafkaConfig.Producer.Partitioner = sarama.NewHashPartitioner
	if configuration.UseTLS {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("cannot initialize TLS: %w", err)
		}
		kafkaConfig.Net.TLS.Enable = true
		kafkaConfig.Net.TLS.Config = &tls.Config{RootCAs: rootCAs}
	}
	if err := kafkaConfig.Validate(); err != nil {
		return nil, fmt.Errorf("cannot validate Kafka configuration: %w", err)
	}

	c := Component{
		r:      reporter,
		d:      &dependencies,
		config: configuration,

		kafkaConfig: kafkaConfig,
		kafkaTopic:  fmt.Sprintf("%s-v%d", configuration.Topic, flow.CurrentSchemaVersion),
	}
	c.initMetrics()
	c.createKafkaProducer = func() (sarama.AsyncProducer, error) {
		return sarama.NewAsyncProducer(c.config.Brokers, c.kafkaConfig)
	}
	c.d.Daemon.Track(&c.t, "kafka")
	return &c, nil
}

// Start starts the Kafka component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting Kafka component")
	globalKafkaLogger.r.Store(c.r)

	// Create producer
	kafkaProducer, err := c.createKafkaProducer()
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to create async producer")
		return fmt.Errorf("unable to create Kafka async producer: %w", err)
	}
	c.kafkaProducer = kafkaProducer

	// Create topic
	if c.config.TopicConfiguration != nil {
		client, err := sarama.NewClusterAdmin(c.config.Brokers, c.kafkaConfig)
		if err != nil {
			kafkaProducer.Close()
			c.r.Err(err).
				Str("brokers", strings.Join(c.config.Brokers, ",")).
				Msg("unable to get admin client for topic creation")
			return fmt.Errorf("unable to get admin client for topic creation: %w", err)
		}
		defer client.Close()
		l := c.r.With().
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Str("topic", c.kafkaTopic).
			Logger()
		topics, err := client.ListTopics()
		if err != nil {
			l.Err(err).Msg("unable to get metadata for topics")
			return fmt.Errorf("unable to get metadata for topics: %w", err)
		}
		if topic, ok := topics[c.kafkaTopic]; !ok {
			if err := client.CreateTopic(c.kafkaTopic,
				&sarama.TopicDetail{
					NumPartitions:     c.config.TopicConfiguration.NumPartitions,
					ReplicationFactor: c.config.TopicConfiguration.ReplicationFactor,
					ConfigEntries:     c.config.TopicConfiguration.ConfigEntries,
				}, false); err != nil {
				l.Err(err).Msg("unable to create topic")
				return fmt.Errorf("unable to create topic %q: %w", c.kafkaTopic, err)
			}
			l.Info().Msg("topic created")
		} else {
			if topic.NumPartitions != c.config.TopicConfiguration.NumPartitions {
				l.Warn().Msgf("mismatch for number of partitions: got %d, want %d",
					topic.NumPartitions, c.config.TopicConfiguration.NumPartitions)
			}
			if topic.ReplicationFactor != c.config.TopicConfiguration.ReplicationFactor {
				l.Warn().Msgf("mismatch for replication factor: got %d, want %d",
					topic.ReplicationFactor, c.config.TopicConfiguration.ReplicationFactor)
			}
			if err := client.AlterConfig(sarama.TopicResource, c.kafkaTopic, c.config.TopicConfiguration.ConfigEntries, false); err != nil {
				l.Err(err).Msg("unable to set topic configuration")
				return fmt.Errorf("unable to set topic configuration for %q: %w",
					c.kafkaTopic, err)
			}
			l.Info().Msg("topic updated")
		}
	}

	// Main loop
	c.t.Go(func() error {
		defer kafkaProducer.Close()
		defer c.kafkaConfig.MetricRegistry.UnregisterAll()
		errLimiter := rate.NewLimiter(rate.Every(10*time.Second), 3)
		for {
			select {
			case <-c.t.Dying():
				c.r.Debug().Msg("stop error logger")
				return nil
			case msg := <-kafkaProducer.Errors():
				c.metrics.errors.WithLabelValues(msg.Error()).Inc()
				if errLimiter.Allow() {
					c.r.Err(msg.Err).
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
	var noreporter *reporter.Reporter
	defer globalKafkaLogger.r.Store(noreporter)
	c.r.Info().Msg("stopping Kafka component")
	defer c.r.Info().Msg("Kafka component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}

// Send a message to Kafka.
func (c *Component) Send(sampler string, payload []byte) {
	c.metrics.bytesSent.WithLabelValues(sampler).Add(float64(len(payload)))
	c.metrics.messagesSent.WithLabelValues(sampler).Inc()
	c.kafkaProducer.Input() <- &sarama.ProducerMessage{
		Topic: c.kafkaTopic,
		Key:   sarama.StringEncoder(sampler),
		Value: sarama.ByteEncoder(payload),
	}
}
