// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// newFakeKafka spins up an in-process fake broker and returns an admin client
// (to assert on topic state) plus its broker addresses. Everything is torn down
// on test cleanup.
func newFakeKafka(t *testing.T) (*kadm.Client, []string) {
	t.Helper()
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.WithLogger(kafka.NewLogger(reporter.NewMock(t))),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	t.Cleanup(func() { cluster.Close() })
	client, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	t.Cleanup(func() { client.Close() })
	return kadm.NewClient(client), cluster.ListenAddrs()
}

func mustListTopic(t *testing.T, admin *kadm.Client, name string) kadm.TopicDetail {
	t.Helper()
	topics, err := admin.ListTopics(t.Context())
	if err != nil {
		t.Fatalf("ListTopics() error:\n%+v", err)
	}
	td, ok := topics[name]
	if !ok {
		t.Fatalf("topic %q was not created", name)
	}
	return td
}

// TestManageInputTopicFake drives the input-topic reconciler against a fake
// broker (no external Kafka needed): create, then re-run to exercise the
// existing-topic path — the decrease-is-refused warning, an unchanged no-op, and
// a configuration change.
func TestManageInputTopicFake(t *testing.T) {
	admin, brokers := newFakeKafka(t)
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expected := fmt.Sprintf("%s-v%d", topicName, pb.Version)
	retentionMs := "76548"
	retentionMs2 := "999999"

	start := func(numPartitions int32, entries map[string]*string) {
		configuration := DefaultConfiguration()
		configuration.Topic = topicName
		configuration.Brokers = brokers
		configuration.ManageTopic = true
		configuration.TopicConfiguration = TopicConfiguration{
			NumPartitions:           numPartitions,
			ReplicationFactor:       1,
			ConfigEntries:           entries,
			ConfigEntriesStrictSync: true,
		}
		c, err := New(reporter.NewMock(t), configuration, nil, Dependencies{Schema: schema.NewMock(t)})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, c)
	}

	configOf := func(key string) string {
		configs, err := admin.DescribeTopicConfigs(t.Context(), expected)
		if err != nil || len(configs) != 1 {
			t.Fatalf("DescribeTopicConfigs() error: %v (len %d)", err, len(configs))
		}
		for _, c := range configs[0].Configs {
			if c.Key == key && c.Value != nil {
				return *c.Value
			}
		}
		return ""
	}

	// Create with 4 partitions.
	start(4, map[string]*string{"retention.ms": &retentionMs})
	if td := mustListTopic(t, admin, expected); len(td.Partitions) != 4 {
		t.Fatalf("got %d partitions, want 4", len(td.Partitions))
	}

	// Ask for fewer partitions with the same config: decrease is refused (warning
	// only) and nothing is altered; the count stays at 4.
	start(2, map[string]*string{"retention.ms": &retentionMs})
	if td := mustListTopic(t, admin, expected); len(td.Partitions) != 4 {
		t.Fatalf("got %d partitions after decrease request, want 4", len(td.Partitions))
	}

	// Change a config value: the alter path runs and the new value sticks.
	start(4, map[string]*string{"retention.ms": &retentionMs2})
	if got := configOf("retention.ms"); got != retentionMs2 {
		t.Fatalf("retention.ms = %q after alter, want %q", got, retentionMs2)
	}

	// Ask for more partitions: the increase path runs (CreatePartitions) without
	// error. The fake broker does not actually grow the topic, so we only assert
	// Start succeeded rather than the resulting count.
	start(8, map[string]*string{"retention.ms": &retentionMs2})
}

// TestManageOutputTopicFake drives the kafka-out output-topic reconciler against
// a fake broker: the output topic is created (schema-suffixed) while the input
// topic is left untouched because ManageTopic is off.
func TestManageOutputTopicFake(t *testing.T) {
	admin, brokers := newFakeKafka(t)
	sch := schema.NewMock(t)
	inputBase := fmt.Sprintf("test-input-%d", rand.Int())
	outputBase := fmt.Sprintf("test-output-%d", rand.Int())
	retentionMs := "76548"

	configuration := DefaultConfiguration()
	configuration.Topic = inputBase
	configuration.Brokers = brokers
	configuration.ManageTopic = false
	output := &OutputConfiguration{
		Configuration: kafka.Configuration{Topic: outputBase, Brokers: brokers},
		TopicConfiguration: TopicConfiguration{
			NumPartitions:     1,
			ReplicationFactor: 1,
			ConfigEntries:     map[string]*string{"retention.ms": &retentionMs},
		},
	}
	c, err := New(reporter.NewMock(t), configuration, output, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil despite kafka-out set")
	}
	helpers.StartStop(t, c)

	expectedOutput := fmt.Sprintf("%s-%s", outputBase, sch.ProtobufMessageHash())
	mustListTopic(t, admin, expectedOutput)

	topics, err := admin.ListTopics(t.Context())
	if err != nil {
		t.Fatalf("ListTopics() error:\n%+v", err)
	}
	unexpectedInput := fmt.Sprintf("%s-v%d", inputBase, pb.Version)
	if _, ok := topics[unexpectedInput]; ok {
		t.Fatalf("input topic %q was created despite ManageTopic=false", unexpectedInput)
	}
}
