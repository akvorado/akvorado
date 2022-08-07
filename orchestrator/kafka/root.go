// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles Kafka-related configuration for the orchestrator.
package kafka

import (
	"fmt"
	"strings"

	"github.com/Shopify/sarama"

	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/inlet/flow"
)

// Component represents the Kafka configurator.
type Component struct {
	r      *reporter.Reporter
	config Configuration

	kafkaConfig *sarama.Config
	kafkaTopic  string
}

// New creates a new Kafka configurator.
func New(r *reporter.Reporter, config Configuration) (*Component, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.KafkaVersion(config.Version)
	if err := kafkaConfig.Validate(); err != nil {
		return nil, fmt.Errorf("cannot validate Kafka configuration: %w", err)
	}

	return &Component{
		r:      r,
		config: config,

		kafkaConfig: kafkaConfig,
		kafkaTopic:  fmt.Sprintf("%s-v%d", config.Topic, flow.CurrentSchemaVersion),
	}, nil
}

// Start starts Kafka configuration.
func (c *Component) Start() error {
	c.r.Info().Msg("starting Kafka component")
	kafka.GlobalKafkaLogger.Register(c.r)
	defer func() {
		kafka.GlobalKafkaLogger.Unregister()
		c.r.Info().Msg("Kafka component stopped")
	}()

	// Create topic
	admin, err := sarama.NewClusterAdmin(c.config.Brokers, c.kafkaConfig)
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to get admin client for topic creation")
		return fmt.Errorf("unable to get admin client for topic creation: %w", err)
	}
	defer admin.Close()
	l := c.r.With().
		Str("brokers", strings.Join(c.config.Brokers, ",")).
		Str("topic", c.kafkaTopic).
		Logger()
	topics, err := admin.ListTopics()
	if err != nil {
		l.Err(err).Msg("unable to get metadata for topics")
		return fmt.Errorf("unable to get metadata for topics: %w", err)
	}
	if topic, ok := topics[c.kafkaTopic]; !ok {
		if err := admin.CreateTopic(c.kafkaTopic,
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
		if topic.NumPartitions > c.config.TopicConfiguration.NumPartitions {
			l.Warn().Msgf("cannot decrease the number of partitions (from %d to %d)",
				topic.NumPartitions, c.config.TopicConfiguration.NumPartitions)
		} else if topic.NumPartitions < c.config.TopicConfiguration.NumPartitions {
			nb := c.config.TopicConfiguration.NumPartitions
			if err := admin.CreatePartitions(c.kafkaTopic, nb, nil, false); err != nil {
				l.Err(err).Msg("unable to add more partitions")
				return fmt.Errorf("unable to add more partitions to topic %q: %w",
					c.kafkaTopic, err)
			}
		}
		if c.config.TopicConfiguration.ReplicationFactor != topic.ReplicationFactor {
			// TODO: https://github.com/deviceinsight/kafkactl/blob/main/internal/topic/topic-operation.go
			l.Warn().Msgf("mismatch for replication factor: got %d, want %d",
				topic.ReplicationFactor, c.config.TopicConfiguration.ReplicationFactor)
		}
		if err := admin.AlterConfig(sarama.TopicResource, c.kafkaTopic, c.config.TopicConfiguration.ConfigEntries, false); err != nil {
			l.Err(err).Msg("unable to set topic configuration")
			return fmt.Errorf("unable to set topic configuration for %q: %w",
				c.kafkaTopic, err)
		}
		l.Info().Msg("topic updated")
	}
	return nil
}
