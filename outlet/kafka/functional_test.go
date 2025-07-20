// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func TestFakeKafka(t *testing.T) {
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(16, expectedTopicName),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	defer cluster.Close()

	// Create a producer client
	producerConfiguration := kafka.DefaultConfiguration()
	producerConfiguration.Brokers = cluster.ListenAddrs()
	producerOpts, err := kafka.NewConfig(reporter.NewMock(t), producerConfiguration)
	if err != nil {
		t.Fatalf("NewConfig() error:\n%+v", err)
	}
	producer, err := kgo.NewClient(producerOpts...)
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	defer producer.Close()

	// Callback
	got := []string{}
	expected := []string{"hello 1", "hello 2", "hello 3"}
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
	configuration.Brokers = cluster.ListenAddrs()
	configuration.FetchMaxWaitTime = 100 * time.Millisecond
	configuration.ConsumerGroup = fmt.Sprintf("outlet-%d", rand.Int())
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

	// Send messages
	time.Sleep(100 * time.Millisecond)
	t.Log("producing values")
	for _, value := range expected {
		record := &kgo.Record{
			Topic: expectedTopicName,
			Value: []byte(value),
		}
		results := producer.ProduceSync(context.Background(), record)
		if err := results.FirstErr(); err != nil {
			t.Fatalf("ProduceSync() error:\n%+v", err)
		}
	}
	t.Log("values produced")

	// Wait for them
	select {
	case <-time.After(time.Second):
		t.Fatal("Too long to get messages")
	case <-gotAll:
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}

	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "received_")
	fetches, _ := strconv.Atoi(gotMetrics[`received_fetches_total{worker="0"}`])
	expectedMetrics := map[string]string{
		`received_bytes_total{worker="0"}`:    "21",
		`received_fetches_total{worker="0"}`:  strconv.Itoa(max(fetches, 1)),
		`received_messages_total{worker="0"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Errorf("Metrics (-got, +want):\n%s", diff)
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}
	if !shutdownCalled {
		t.Fatal("Stop() didn't call shutdown function")
	}
}

func TestStartSeveralWorkers(t *testing.T) {
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(16, expectedTopicName),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	defer cluster.Close()

	// Start the component
	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = cluster.ListenAddrs()
	configuration.FetchMaxWaitTime = 100 * time.Millisecond
	configuration.ConsumerGroup = fmt.Sprintf("outlet-%d", rand.Int())
	configuration.Workers = 5
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.(*realComponent).Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	c.StartWorkers(func(int) (ReceiveFunc, ShutdownFunc) {
		return func(context.Context, []byte) error { return nil }, func() {}
	})
	time.Sleep(20 * time.Millisecond)
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}

	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_")
	connectsTotal := 0
	writeBytesTotal := 0
	readBytesTotal := 0
	for k := range gotMetrics {
		if strings.HasPrefix(k, "write_bytes_total") {
			writeBytesTotal++
		}
		if strings.HasPrefix(k, "read_bytes_total") {
			readBytesTotal++
		}
		if strings.HasPrefix(k, "connects_total") {
			connectsTotal++
		}
	}
	got := map[string]int{
		"write_bytes_total": writeBytesTotal,
		"read_bytes_total":  readBytesTotal,
		"connects_total":    connectsTotal,
	}
	expected := map[string]int{
		// For some reason, we have each metric in double, with one seed_0.
		"write_bytes_total": 10,
		"read_bytes_total":  10,
		"connects_total":    10,
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Metrics (-got, +want):\n%s", diff)
	}
}
