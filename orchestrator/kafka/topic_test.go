// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/twmb/franz-go/pkg/kfake"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// newFakeKafka spins up an in-process fake broker. It is torn down on test
// cleanup.
func newFakeKafka(t *testing.T) *kfake.Cluster {
	t.Helper()
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.WithLogger(kafka.NewLogger(reporter.NewMock(t))),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	t.Cleanup(func() { cluster.Close() })
	return cluster
}

func topicPartitions(t *testing.T, cluster *kfake.Cluster, name string) int {
	t.Helper()
	if cluster.TopicInfo(name) == nil {
		t.Fatalf("topic %q was not created", name)
	}
	return len(cluster.PartitionInfos(name))
}

// TestManageInputTopicFake drives the input-topic reconciler against a fake
// broker (no external Kafka needed): create, then re-run to exercise the
// existing-topic path — the decrease-is-refused warning, an unchanged no-op, and
// a configuration change.
func TestManageInputTopicFake(t *testing.T) {
	cluster := newFakeKafka(t)
	brokers := cluster.ListenAddrs()
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expected := fmt.Sprintf("%s-v%d", topicName, pb.Version)
	retentionMs := "76548"
	retentionMs2 := "999999"

	start := func(numPartitions int32, entries map[string]*string) {
		configuration := DefaultInputConfiguration()
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
		info := cluster.TopicInfo(expected)
		if info == nil {
			t.Fatalf("topic %q was not created", expected)
		}
		if v := info.Configs[key]; v != nil {
			return *v
		}
		return ""
	}

	// Create with 4 partitions.
	start(4, map[string]*string{"retention.ms": &retentionMs})
	if diff := helpers.Diff(topicPartitions(t, cluster, expected), 4); diff != "" {
		t.Fatalf("Partitions (-got, +want):\n%s", diff)
	}

	// Ask for fewer partitions with the same config: decrease is refused (warning
	// only) and nothing is altered; the count stays at 4.
	start(2, map[string]*string{"retention.ms": &retentionMs})
	if diff := helpers.Diff(topicPartitions(t, cluster, expected), 4); diff != "" {
		t.Fatalf("Partitions after decrease request (-got, +want):\n%s", diff)
	}

	// Change a config value: the alter path runs and the new value sticks.
	start(4, map[string]*string{"retention.ms": &retentionMs2})
	if diff := helpers.Diff(configOf("retention.ms"), retentionMs2); diff != "" {
		t.Fatalf("retention.ms after alter (-got, +want):\n%s", diff)
	}

	// Ask for more partitions: the increase path runs (CreatePartitions).
	start(8, map[string]*string{"retention.ms": &retentionMs2})
	if diff := helpers.Diff(topicPartitions(t, cluster, expected), 8); diff != "" {
		t.Fatalf("Partitions after increase request (-got, +want):\n%s", diff)
	}
}

// TestManageOutputTopicFake drives the kafka-output output-topic reconciler against
// a fake broker: the output topic is created (schema-suffixed) while the input
// topic is left untouched because ManageTopic is off.
func TestManageOutputTopicFake(t *testing.T) {
	cluster := newFakeKafka(t)
	brokers := cluster.ListenAddrs()
	sch := schema.NewMock(t)
	inputBase := fmt.Sprintf("test-input-%d", rand.Int())
	outputBase := fmt.Sprintf("test-output-%d", rand.Int())
	retentionMs := "76548"

	configuration := DefaultInputConfiguration()
	configuration.Topic = inputBase
	configuration.Brokers = brokers
	configuration.ManageTopic = false
	output := &OutputConfiguration{
		Topic: outputBase, Brokers: brokers,
		NumPartitions:     1,
		ReplicationFactor: 1,
		ConfigEntries:     map[string]*string{"retention.ms": &retentionMs},
	}
	c, err := New(reporter.NewMock(t), configuration, output, Dependencies{Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if c == nil {
		t.Fatal("New() returned nil despite kafka-output set")
	}
	helpers.StartStop(t, c)

	expectedOutput := fmt.Sprintf("%s-%s", outputBase, sch.ProtobufMessageHash())
	topicPartitions(t, cluster, expectedOutput)

	unexpectedInput := fmt.Sprintf("%s-v%d", inputBase, pb.Version)
	if cluster.TopicInfo(unexpectedInput) != nil {
		t.Fatalf("input topic %q was created despite ManageTopic=false", unexpectedInput)
	}
}
