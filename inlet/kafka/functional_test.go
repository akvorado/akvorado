// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Shopify/sarama"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func TestRealKafka(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = brokers
	configuration.Version = kafka.Version(sarama.V2_8_1_0)
	configuration.FlushInterval = 100 * time.Millisecond
	expectedTopicName := fmt.Sprintf("%s-%s", topicName, schema.Flows.ProtobufMessageHash())
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	c.Send("127.0.0.1", []byte("hello world!"))
	c.Send("127.0.0.1", []byte("goodbye world!"))

	time.Sleep(10 * time.Millisecond)
	gotMetrics := r.GetMetrics("akvorado_inlet_kafka_", "sent_")
	expectedMetrics := map[string]string{
		`sent_bytes_total{exporter="127.0.0.1"}`:    "26",
		`sent_messages_total{exporter="127.0.0.1"}`: "2",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Try to consume the two messages
	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		t.Fatalf("NewConsumerGroup() error:\n%+v", err)
	}
	defer consumer.Close()
	var partitions []int32
	for {
		partitions, err = consumer.Partitions(expectedTopicName)
		if err != nil {
			if errors.Is(err, sarama.ErrUnknownTopicOrPartition) {
				// Wait for topic to be available
				continue
			}
			t.Fatalf("Partitions() error:\n%+v", err)
		}
		break
	}
	partitionConsumer, err := consumer.ConsumePartition(expectedTopicName, partitions[0], sarama.OffsetOldest)
	if err != nil {
		t.Fatalf("ConsumePartitions() error:\n%+v", err)
	}

	got := []string{}
	expected := []string{
		"hello world!",
		"goodbye world!",
	}
	timeout := time.After(15 * time.Second)
	for i := 0; i < len(expected); i++ {
		select {
		case msg := <-partitionConsumer.Messages():
			got = append(got, string(msg.Value))
		case err := <-partitionConsumer.Errors():
			t.Fatalf("consumer.Errors():\n%+v", err)
		case <-timeout:
		}
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}
}
