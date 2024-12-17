// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/IBM/sarama"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func TestRealKafka(t *testing.T) {
	client, brokers := kafka.SetupKafkaBroker(t)

	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)
	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = brokers
	configuration.Version = kafka.Version(sarama.V2_8_1_0)
	configuration.FlushInterval = 100 * time.Millisecond
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	msg1 := make([]byte, 50)
	msg2 := make([]byte, 50)
	for i := range msg1 {
		msg1[i] = letters[rand.Intn(len(letters))]
	}
	for i := range msg2 {
		msg1[i] = letters[rand.Intn(len(letters))]
	}
	c.Send("127.0.0.1", msg1)
	c.Send("127.0.0.1", msg2)

	time.Sleep(10 * time.Millisecond)
	gotMetrics := r.GetMetrics("akvorado_inlet_kafka_", "sent_")
	expectedMetrics := map[string]string{
		`sent_bytes_total{exporter="127.0.0.1"}`:    "100",
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
	expected := []string{string(msg1), string(msg2)}
	timeout := time.After(15 * time.Second)
	for range len(expected) {
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
