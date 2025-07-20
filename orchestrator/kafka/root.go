// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka handles Kafka-related configuration for the orchestrator.
package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// Component represents the Kafka configurator.
type Component struct {
	r      *reporter.Reporter
	d      Dependencies
	config Configuration

	kafkaOpts  []kgo.Opt
	kafkaTopic string
}

// Dependencies are the dependencies for the Kafka component
type Dependencies struct {
	Schema *schema.Component
}

// New creates a new Kafka configurator.
func New(r *reporter.Reporter, config Configuration, dependencies Dependencies) (*Component, error) {
	kafkaOpts, err := kafka.NewConfig(r, config.Configuration)
	if err != nil {
		return nil, err
	}

	c := Component{
		r:      r,
		d:      dependencies,
		config: config,

		kafkaOpts:  kafkaOpts,
		kafkaTopic: fmt.Sprintf("%s-v%d", config.Topic, pb.Version),
	}
	return &c, nil
}

// Start starts Kafka configuration.
func (c *Component) Start() error {
	c.r.Info().Msg("starting Kafka component")
	defer c.r.Info().Msg("Kafka component stopped")

	// Create kafka client and admin
	client, err := kgo.NewClient(c.kafkaOpts...)
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(c.config.Brokers, ",")).
			Msg("unable to create Kafka client for topic creation")
		return fmt.Errorf("unable to create Kafka client for topic creation: %w", err)
	}
	defer client.Close()
	admin := kadm.NewClient(client)
	l := c.r.With().
		Str("brokers", strings.Join(c.config.Brokers, ",")).
		Str("topic", c.kafkaTopic).
		Logger()
	topics, err := admin.ListTopics(context.Background())
	if err != nil {
		l.Err(err).Msg("unable to get metadata for topics")
		return fmt.Errorf("unable to get metadata for topics: %w", err)
	}
	if topic, ok := topics[c.kafkaTopic]; !ok {
		if _, err := admin.CreateTopics(context.Background(), c.config.TopicConfiguration.NumPartitions, c.config.TopicConfiguration.ReplicationFactor, c.config.TopicConfiguration.ConfigEntries, c.kafkaTopic); err != nil {
			l.Err(err).Msg("unable to create topic")
			return fmt.Errorf("unable to create topic %q: %w", c.kafkaTopic, err)
		}
		l.Info().Msg("topic created")
	} else {
		nbPartitions := len(topic.Partitions)
		if nbPartitions > int(c.config.TopicConfiguration.NumPartitions) {
			l.Warn().Msgf("cannot decrease the number of partitions (from %d to %d)",
				nbPartitions, c.config.TopicConfiguration.NumPartitions)
		} else if nbPartitions < int(c.config.TopicConfiguration.NumPartitions) {
			add := int(c.config.TopicConfiguration.NumPartitions) - nbPartitions
			if _, err := admin.CreatePartitions(context.Background(), add, c.kafkaTopic); err != nil {
				l.Err(err).Msg("unable to add more partitions")
				return fmt.Errorf("unable to add more partitions to topic %q: %w",
					c.kafkaTopic, err)
			}
			l.Info().Msg("number of partitions increased")
		}
		if int(c.config.TopicConfiguration.ReplicationFactor) != topic.Partitions.NumReplicas() {
			// TODO: https://github.com/deviceinsight/kafkactl/blob/main/internal/topic/topic-operation.go
			l.Warn().Msgf("mismatch for replication factor: got %d, want %d",
				topic.Partitions.NumReplicas(), c.config.TopicConfiguration.ReplicationFactor)
		}
		configs, err := admin.DescribeTopicConfigs(context.Background(), c.kafkaTopic)
		if err != nil || len(configs) != 1 {
			l.Err(err).Msg("unable to get topic configuration")
			return fmt.Errorf("unable to get topic %q configuration: %w", c.kafkaTopic, err)
		}
		got := map[string]*string{}
		for _, config := range configs[0].Configs {
			if config.Source == kmsg.ConfigSourceDynamicTopicConfig {
				got[config.Key] = config.Value
			}
		}
		if ShouldAlterConfiguration(c.config.TopicConfiguration.ConfigEntries, got, c.config.TopicConfiguration.ConfigEntriesStrictSync) {
			alterConfigs := []kadm.AlterConfig{}
			for k, v := range c.config.TopicConfiguration.ConfigEntries {
				alterConfigs = append(alterConfigs, kadm.AlterConfig{
					Op:    kadm.SetConfig,
					Name:  k,
					Value: v,
				})
			}
			for k, v := range got {
				if _, ok := c.config.TopicConfiguration.ConfigEntries[k]; !ok {
					alterConfigs = append(alterConfigs, kadm.AlterConfig{
						Op:    kadm.DeleteConfig,
						Name:  k,
						Value: v,
					})
				}
			}
			if _, err := admin.AlterTopicConfigs(context.Background(), alterConfigs, c.kafkaTopic); err != nil {
				l.Err(err).Msg("unable to set topic configuration")
				return fmt.Errorf("unable to set topic configuration for %q: %w",
					c.kafkaTopic, err)
			}
			l.Info().Msg("topic updated")
		}
	}
	return nil
}
