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
	"akvorado/inlet/flow"
)

func TestTopicCreation(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, flow.CurrentSchemaVersion)
	retentionMs := "76548"
	segmentBytes := "107374184"
	segmentBytes2 := "10737184"
	cleanupPolicy := "delete"

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
			configuration := DefaultConfiguration
			configuration.Connect.Topic = topicName
			configuration.TopicConfiguration = TopicConfiguration{
				NumPartitions:     1,
				ReplicationFactor: 1,
				ConfigEntries:     tc.ConfigEntries,
			}
			configuration.Connect.Brokers = brokers
			configuration.Connect.Version = kafka.Version(sarama.V2_8_1_0)
			c, err := New(reporter.NewMock(t), configuration)
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			if err := c.Start(); err != nil {
				t.Fatalf("Start() error:\n%+v", err)
			}

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
