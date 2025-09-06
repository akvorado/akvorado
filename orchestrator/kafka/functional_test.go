// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestTopicCreation(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)
	adminClient := kadm.NewClient(client)

	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	retentionMs := "76548"
	segmentBytes := "107374184"
	segmentBytes2 := "10737184"
	cleanupPolicy := "delete"
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

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
			Name: "Do not alter equivalent config",
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
				NumPartitions:           1,
				ReplicationFactor:       1,
				ConfigEntries:           tc.ConfigEntries,
				ConfigEntriesStrictSync: true,
			}
			configuration.Brokers = brokers
			// No version configuration needed for franz-go
			c, err := New(reporter.NewMock(t), configuration, Dependencies{Schema: schema.NewMock(t)})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, c)

			deadline := time.Now().Add(1 * time.Second)
			for {
				topics, err := adminClient.ListTopics(t.Context())
				if err != nil {
					t.Fatalf("ListTopics() error:\n%+v", err)
				}
				_, ok := topics[expectedTopicName]
				if !ok {
					if time.Now().Before(deadline) {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					t.Fatal("ListTopics() did not find the topic")
				}
				configs, err := adminClient.DescribeTopicConfigs(context.Background(), c.kafkaTopic)
				if err != nil {
					t.Fatalf("DescribeTopicConfigs() error:\n%+v", err)
				}
				got := map[string]*string{}
				for _, config := range configs[0].Configs {
					if config.Source != kmsg.ConfigSourceDefaultConfig && config.Key != "min.insync.replicas" {
						got[config.Key] = config.Value
					}
				}
				if diff := helpers.Diff(got, tc.ConfigEntries); diff != "" {
					if time.Now().Before(deadline) {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					t.Fatalf("ListTopics() (-got, +want):\n%s", diff)
				}
				break
			}
		})
	}
}

func TestTopicMorePartitions(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)
	adminClient := kadm.NewClient(client)

	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.TopicConfiguration = TopicConfiguration{
		NumPartitions:     1,
		ReplicationFactor: 1,
		ConfigEntries:     map[string]*string{},
	}

	configuration.Brokers = brokers
	// No version configuration needed for franz-go
	c, err := New(reporter.NewMock(t), configuration, Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	deadline := time.Now().Add(1 * time.Second)
	for {
		topics, err := adminClient.ListTopics(t.Context())
		if err != nil {
			t.Fatalf("ListTopics() error:\n%+v", err)
		}
		topic, ok := topics[expectedTopicName]
		if !ok {
			if time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.Fatal("ListTopics() did not find the topic")
		}
		if len(topic.Partitions) != 1 || topic.Partitions.NumReplicas() != 1 {
			if time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.Fatalf("Topic does not have 1/1 for partitions/replication but %d/%d",
				len(topic.Partitions), topic.Partitions.NumReplicas())
		}
		break
	}

	// Increase number of partitions
	configuration.TopicConfiguration.NumPartitions = 4
	c, err = New(reporter.NewMock(t), configuration, Dependencies{Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	deadline = time.Now().Add(1 * time.Second)
	for {
		topics, err := adminClient.ListTopics(t.Context())
		if err != nil {
			t.Fatalf("ListTopics() error:\n%+v", err)
		}
		topic := topics[expectedTopicName]
		t.Logf("Topic configuration:\n%+v", topic)
		if len(topic.Partitions) != 4 || topic.Partitions.NumReplicas() != 1 {
			if time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.Fatalf("Topic does not have 4/1 for partitions/replication but %d/%d",
				len(topic.Partitions), topic.Partitions.NumReplicas())
		}
		break
	}
}
