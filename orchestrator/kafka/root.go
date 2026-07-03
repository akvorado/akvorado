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

	kafkaOpts   []kgo.Opt
	kafkaTopic  string
	output      *OutputConfiguration
	outputOpts  []kgo.Opt
	outputTopic string
}

// Dependencies are the dependencies for the Kafka component
type Dependencies struct {
	Schema *schema.Component
}

// New creates a new Kafka configurator.
func New(r *reporter.Reporter, config Configuration, output *OutputConfiguration, dependencies Dependencies) (*Component, error) {
	if !config.ManageTopic && output == nil {
		r.Info().Msg("Kafka topic management disabled, skipping Kafka initialization")
		return nil, nil
	}

	c := Component{
		r:      r,
		d:      dependencies,
		config: config,
		output: output,
	}
	if config.ManageTopic {
		kafkaOpts, err := kafka.NewConfig(r, config.Configuration)
		if err != nil {
			return nil, err
		}
		c.kafkaOpts = kafkaOpts
		c.kafkaTopic = fmt.Sprintf("%s-v%d", config.Topic, pb.Version)
	}
	if output != nil {
		// The output topic uses its own connection (possibly a different cluster
		// than the input) and mirrors the outlet's kafka-out naming: base name
		// plus the schema hash, so an incompatible schema change lands on a new
		// topic.
		outputOpts, err := kafka.NewConfig(r, output.Configuration)
		if err != nil {
			return nil, err
		}
		c.outputOpts = outputOpts
		c.outputTopic = fmt.Sprintf("%s-%s", output.Topic, dependencies.Schema.ProtobufMessageHash())
	}
	return &c, nil
}

// Start starts Kafka configuration.
func (c *Component) Start() error {
	if c == nil {
		return nil
	}
	c.r.Info().Msg("starting Kafka component")
	defer c.r.Info().Msg("Kafka component stopped")

	if c.config.ManageTopic {
		if err := c.manage(c.kafkaOpts, c.config.Brokers, c.kafkaTopic, c.config.TopicConfiguration); err != nil {
			return err
		}
	}
	if c.output != nil {
		if err := c.manage(c.outputOpts, c.output.Brokers, c.outputTopic, c.output.TopicConfiguration); err != nil {
			return err
		}
	}
	return nil
}

// manage connects to a cluster with the given client options and creates or
// reconciles the topic there. brokers is used only for log context.
func (c *Component) manage(opts []kgo.Opt, brokers []string, topic string, tc TopicConfiguration) error {
	client, err := kgo.NewClient(opts...)
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(brokers, ",")).
			Msg("unable to create Kafka client for topic creation")
		return fmt.Errorf("unable to create Kafka client for topic creation: %w", err)
	}
	defer client.Close()
	admin := kadm.NewClient(client)
	topics, err := admin.ListTopics(context.Background())
	if err != nil {
		c.r.Err(err).
			Str("brokers", strings.Join(brokers, ",")).
			Msg("unable to get metadata for topics")
		return fmt.Errorf("unable to get metadata for topics: %w", err)
	}
	return c.reconcileTopic(admin, topics, brokers, topic, tc)
}

// reconcileTopic creates the topic if it is missing, or aligns its partition
// count and configuration if it already exists.
func (c *Component) reconcileTopic(admin *kadm.Client, topics kadm.TopicDetails, brokers []string, topic string, tc TopicConfiguration) error {
	l := c.r.With().
		Str("brokers", strings.Join(brokers, ",")).
		Str("topic", topic).
		Logger()
	td, ok := topics[topic]
	if !ok {
		if _, err := admin.CreateTopics(context.Background(), tc.NumPartitions, tc.ReplicationFactor, tc.ConfigEntries, topic); err != nil {
			l.Err(err).Msg("unable to create topic")
			return fmt.Errorf("unable to create topic %q: %w", topic, err)
		}
		l.Info().Msg("topic created")
		return nil
	}
	nbPartitions := len(td.Partitions)
	if nbPartitions > int(tc.NumPartitions) {
		l.Warn().Msgf("cannot decrease the number of partitions (from %d to %d)",
			nbPartitions, tc.NumPartitions)
	} else if nbPartitions < int(tc.NumPartitions) {
		add := int(tc.NumPartitions) - nbPartitions
		if _, err := admin.CreatePartitions(context.Background(), add, topic); err != nil {
			l.Err(err).Msg("unable to add more partitions")
			return fmt.Errorf("unable to add more partitions to topic %q: %w", topic, err)
		}
		l.Info().Msg("number of partitions increased")
	}
	if int(tc.ReplicationFactor) != td.Partitions.NumReplicas() {
		// TODO: https://github.com/deviceinsight/kafkactl/blob/main/internal/topic/topic-operation.go
		l.Warn().Msgf("mismatch for replication factor: got %d, want %d",
			td.Partitions.NumReplicas(), tc.ReplicationFactor)
	}
	configs, err := admin.DescribeTopicConfigs(context.Background(), topic)
	if err != nil || len(configs) != 1 {
		l.Err(err).Msg("unable to get topic configuration")
		return fmt.Errorf("unable to get topic %q configuration: %w", topic, err)
	}
	got := map[string]*string{}
	for _, config := range configs[0].Configs {
		if config.Source == kmsg.ConfigSourceDynamicTopicConfig {
			got[config.Key] = config.Value
		}
	}
	if ShouldAlterConfiguration(tc.ConfigEntries, got, tc.ConfigEntriesStrictSync) {
		alterConfigs := []kadm.AlterConfig{}
		for k, v := range tc.ConfigEntries {
			alterConfigs = append(alterConfigs, kadm.AlterConfig{
				Op:    kadm.SetConfig,
				Name:  k,
				Value: v,
			})
		}
		for k, v := range got {
			if _, ok := tc.ConfigEntries[k]; !ok {
				alterConfigs = append(alterConfigs, kadm.AlterConfig{
					Op:    kadm.DeleteConfig,
					Name:  k,
					Value: v,
				})
			}
		}
		if _, err := admin.AlterTopicConfigs(context.Background(), alterConfigs, topic); err != nil {
			l.Err(err).Msg("unable to set topic configuration")
			return fmt.Errorf("unable to set topic configuration for %q: %w", topic, err)
		}
		l.Info().Msg("topic updated")
	}
	return nil
}
