// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Shopify/sarama"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestTopicCreation(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	retentionMs := "76548"
	segmentBytes := "107374184"
	segmentBytes2 := "10737184"
	cleanupPolicy := "delete"
	expectedTopicName := fmt.Sprintf("%s-%s", topicName, schema.NewMock(t).ProtobufMessageHash())

	cases := []struct {
		Name          string
		ConfigEntries map[string]*string
	}{
		{
			Name: "Set initial config",
			ConfigEntries: map[string]*string{
				"retention.ms":  &retentionMs,
				"segment.bytes": &segmentBytes,
			},
		}, {
			Name: "Alter initial config",
			ConfigEntries: map[string]*string{
				"retention.ms":   &retentionMs,
				"segment.bytes":  &segmentBytes2,
				"cleanup.policy": &cleanupPolicy,
			},
		}, {
			Name: "Remove item",
			ConfigEntries: map[string]*string{
				"retention.ms":  &retentionMs,
				"segment.bytes": &segmentBytes2,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			configuration := DefaultConfiguration()
			configuration.Topic = topicName
			configuration.TopicConfiguration = TopicConfiguration{
				NumPartitions:     1,
				ReplicationFactor: 1,
				ConfigEntries:     tc.ConfigEntries,
			}
			configuration.Brokers = brokers
			configuration.Version = kafka.Version(sarama.V2_8_1_0)
			c, err := New(reporter.NewMock(t), configuration, Dependencies{Schema: schema.NewMock(t)})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, c)

			adminClient, err := sarama.NewClusterAdminFromClient(client)
			if err != nil {
				t.Fatalf("NewClusterAdmin() error:\n%+v", err)
			}
			topics, err := adminClient.ListTopics()
			if err != nil {
				t.Fatalf("ListTopics() error:\n%+v", err)
			}
			topic, ok := topics[expectedTopicName]
			if !ok {
				t.Fatal("ListTopics() did not find the topic")
			}
			if diff := helpers.Diff(topic.ConfigEntries, tc.ConfigEntries); diff != "" {
				t.Fatalf("ListTopics() (-got, +want):\n%s", diff)
			}
		})
	}

}

func TestTopicMorePartitions(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-%s", topicName, schema.NewMock(t).ProtobufMessageHash())

	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.TopicConfiguration = TopicConfiguration{
		NumPartitions:     1,
		ReplicationFactor: 1,
		ConfigEntries:     map[string]*string{},
	}

	configuration.Brokers = brokers
	configuration.Version = kafka.Version(sarama.V2_8_1_0)
	c, err := New(reporter.NewMock(t), configuration, Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	adminClient, err := sarama.NewClusterAdminFromClient(client)
	if err != nil {
		t.Fatalf("NewClusterAdmin() error:\n%+v", err)
	}
	topics, err := adminClient.ListTopics()
	if err != nil {
		t.Fatalf("ListTopics() error:\n%+v", err)
	}
	topic, ok := topics[expectedTopicName]
	if !ok {
		t.Fatal("ListTopics() did not find the topic")
	}
	if topic.NumPartitions != 1 || topic.ReplicationFactor != 1 {
		t.Fatalf("Topic does not have 1/1 for partitions/replication but %d/%d",
			topic.NumPartitions, topic.ReplicationFactor)
	}

	// Increase number of partitions
	configuration.TopicConfiguration.NumPartitions = 4
	c, err = New(reporter.NewMock(t), configuration, Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	topics, err = adminClient.ListTopics()
	if err != nil {
		t.Fatalf("ListTopics() error:\n%+v", err)
	}
	topic = topics[expectedTopicName]
	t.Logf("Topic configuration:\n%+v", topic)
	if topic.NumPartitions != 4 || topic.ReplicationFactor != 1 {
		t.Fatalf("Topic does not have 4/1 for partitions/replication but %d/%d",
			topic.NumPartitions, topic.ReplicationFactor)
	}
}
