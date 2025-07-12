// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
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

	// Create the topic
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)
	admin, err := sarama.NewClusterAdminFromClient(client)
	if err != nil {
		t.Fatalf("NewClusterAdminFromClient() error:\n%+v", err)
	}
	defer admin.Close()
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     1,
		ReplicationFactor: 1,
	}
	err = admin.CreateTopic(expectedTopicName, topicDetail, false)
	if err != nil {
		t.Fatalf("CreateTopic() error:\n%+v", err)
	}

	// Create a producer
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		t.Fatalf("NewSyncProducerFromClient() error:\n%+v", err)
	}
	defer producer.Close()

	// Callback
	got := []string{}
	expected := []string{"hello", "hello 2", "hello 3"}
	gotAll := make(chan bool)
	callback := func(_ context.Context, message []byte) error {
		got = append(got, string(message))
		if len(got) == len(expected) {
			close(gotAll)
		}
		return nil
	}

	// Start the component
	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = brokers
	configuration.Version = kafka.Version(sarama.V2_8_1_0)
	configuration.FetchMaxWaitTime = 100 * time.Millisecond
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.(*realComponent).Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	shutdownCalled := false
	c.StartWorkers(func(_ int) (ReceiveFunc, ShutdownFunc) { return callback, func() { shutdownCalled = true } })

	// Wait for a claim to be processed. Due to rebalance, it could take more than 3 seconds.
	timeout := time.After(10 * time.Second)
	for {
		gotMetrics := r.GetMetrics("akvorado_outlet_kafka_")
		if gotMetrics[`received_claims_total{worker="0"}`] == "1" {
			break
		}
		select {
		case <-timeout:
			t.Fatal("No claim received")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Send messages
	for _, value := range expected {
		msg := &sarama.ProducerMessage{
			Topic: expectedTopicName,
			Value: sarama.StringEncoder(value),
		}
		if _, _, err := producer.SendMessage(msg); err != nil {
			t.Fatalf("SendMessage() error:\n%+v", err)
		}
	}

	// Wait for them
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Too long to get messages")
	case <-gotAll:
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}

	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "received_")
	expectedMetrics := map[string]string{
		`received_bytes_total{worker="0"}`:    "19",
		`received_claims_total{worker="0"}`:   "1",
		`received_messages_total{worker="0"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Errorf("Metrics (-got, +want):\n%s", diff)
	}

	{
		// Test the healthcheck function
		got := r.RunHealthchecks(context.Background())
		if diff := helpers.Diff(got.Details["kafka"], reporter.HealthcheckResult{
			Status: reporter.HealthcheckOK,
			Reason: "worker 0 ok",
		}); diff != "" {
			t.Fatalf("runHealthcheck() (-got, +want):\n%s", diff)
		}
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}
	if !shutdownCalled {
		t.Fatal("Stop() didn't call shutdown function")
	}
}
