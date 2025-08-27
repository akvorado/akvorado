// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"
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
	c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, ShutdownFunc) {
		return callback, func() { shutdownCalled = true }
	})

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
	configuration.MinWorkers = 5
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.(*realComponent).Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, ShutdownFunc) {
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

func TestWorkerScaling(t *testing.T) {
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

	// Start the component
	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = cluster.ListenAddrs()
	configuration.FetchMaxWaitTime = 10 * time.Millisecond
	configuration.ConsumerGroup = fmt.Sprintf("outlet-%d", rand.Int())
	configuration.WorkerIncreaseRateLimit = 20 * time.Millisecond
	configuration.WorkerDecreaseRateLimit = 20 * time.Millisecond
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	msg := atomic.Uint32{}
	c.StartWorkers(func(_ int, ch chan<- ScaleRequest) (ReceiveFunc, ShutdownFunc) {
		return func(context.Context, []byte) error {
			c := msg.Add(1)
			if c <= 5 || c > 15 {
				t.Logf("received message %d, request a scale increase", c)
				ch <- ScaleIncrease
			} else {
				t.Logf("received message %d, request a scale decrease", c)
				ch <- ScaleDecrease
			}
			return nil
		}, func() {}
	})

	// 1 worker
	time.Sleep(10 * time.Millisecond)
	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "worker")
	expected := map[string]string{
		"worker_decrease_total": "0",
		"worker_increase_total": "1",
		"workers":               "1",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Send 5 messages in a row, expect a second worker
	t.Log("Send 5 messages")
	for range 5 {
		record := &kgo.Record{
			Topic: expectedTopicName,
			Value: []byte("hello"),
		}
		if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
			t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
		}
	}
	time.Sleep(100 * time.Millisecond)
	t.Log("Check if workers increased to 2")
	gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "worker")
	expected = map[string]string{
		"worker_decrease_total": "0",
		"worker_increase_total": "2",
		"workers":               "2",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Send 5 other messages, expect one less worker
	t.Log("Send 5 other messages")
	for range 5 {
		record := &kgo.Record{
			Topic: expectedTopicName,
			Value: []byte("hello"),
		}
		if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
			t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
		}
	}
	time.Sleep(100 * time.Millisecond)
	t.Log("Check if workers decreased to 1")
	gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "worker")
	expected = map[string]string{
		"worker_decrease_total": "1",
		"worker_increase_total": "2",
		"workers":               "1",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Send 5 other messages, expect nothing change (already at minimum)
	t.Log("Send 5 more messages")
	for range 5 {
		record := &kgo.Record{
			Topic: expectedTopicName,
			Value: []byte("hello"),
		}
		if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
			t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
		}
	}
	time.Sleep(100 * time.Millisecond)
	t.Log("Check there is no change in number of workers")
	gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "worker")
	expected = map[string]string{
		"worker_decrease_total": "1",
		"worker_increase_total": "2",
		"workers":               "1",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Send 5 more and expect a scale increase
	t.Log("Send 5 last messages")
	for range 5 {
		record := &kgo.Record{
			Topic: expectedTopicName,
			Value: []byte("hello"),
		}
		if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
			t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
		}
	}
	time.Sleep(100 * time.Millisecond)
	t.Log("Check there are one new worker")
	gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "worker")
	expected = map[string]string{
		"worker_decrease_total": "1",
		"worker_increase_total": "3",
		"workers":               "2",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
