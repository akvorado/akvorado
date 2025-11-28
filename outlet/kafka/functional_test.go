// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func TestFakeKafka(t *testing.T) {
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(16, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
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
	producerOpts = append(producerOpts, kgo.ProducerLinger(0))
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
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(16, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
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

	got := r.GetMetrics("akvorado_outlet_kafka_", "workers")
	expected := map[string]string{
		"workers": "5",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestWorkerStop(t *testing.T) {
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-topic3-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
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
	configuration.MinWorkers = 1
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	var last int
	done := make(chan bool)
	c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, ShutdownFunc) {
		return func(_ context.Context, got []byte) error {
				last, _ = strconv.Atoi(string(got))
				return nil
			}, func() {
				close(done)
			}
	})
	time.Sleep(50 * time.Millisecond)

	// Start producing
	producerConfiguration := kafka.DefaultConfiguration()
	producerConfiguration.Brokers = cluster.ListenAddrs()
	producerOpts, err := kafka.NewConfig(reporter.NewMock(t), producerConfiguration)
	if err != nil {
		t.Fatalf("NewConfig() error:\n%+v", err)
	}
	producerOpts = append(producerOpts, kgo.ProducerLinger(0))
	producer, err := kgo.NewClient(producerOpts...)
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	defer producer.Close()
	produceCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		for i := 1; ; i++ {
			record := &kgo.Record{
				Topic: expectedTopicName,
				Value: []byte(strconv.Itoa(i)),
			}
			producer.ProduceSync(produceCtx, record)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Wait a bit and stop workers
	time.Sleep(500 * time.Millisecond)
	c.StopWorkers()
	select {
	case <-done:
	default:
		t.Fatal("StopWorkers(): worker still running!")
	}
	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "received_messages_total")
	expected := map[string]string{
		`received_messages_total{worker="0"}`: strconv.Itoa(last),
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Check that if we consume from the same group, we will resume from last+1
	consumerConfiguration := kafka.DefaultConfiguration()
	consumerConfiguration.Brokers = cluster.ListenAddrs()
	consumerOpts, err := kafka.NewConfig(reporter.NewMock(t), consumerConfiguration)
	if err != nil {
		t.Fatalf("NewConfig() error:\n%+v", err)
	}
	consumerOpts = append(consumerOpts,
		kgo.ConsumerGroup(configuration.ConsumerGroup),
		kgo.ConsumeTopics(expectedTopicName),
		kgo.FetchMinBytes(1),
		kgo.FetchMaxWait(10*time.Millisecond),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtStart()),
	)
	consumer, err := kgo.NewClient(consumerOpts...)
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	defer consumer.Close()
	fetches := consumer.PollFetches(t.Context())
	if fetches.IsClientClosed() {
		t.Fatal("PollFetches(): client is closed")
	}
	fetches.EachError(func(_ string, _ int32, err error) {
		t.Fatalf("PollFetches() error:\n%+v", err)
	})
	var first int
	fetches.EachRecord(func(r *kgo.Record) {
		if first == 0 {
			first, _ = strconv.Atoi(string(r.Value))
		}
	})
	if last+1 != first {
		t.Fatalf("PollFetches: %d -> %d", last, first)
	}
}

func TestWorkerScaling(t *testing.T) {
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(4, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
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
	producerOpts = append(producerOpts, kgo.ProducerLinger(0))
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
	configuration.WorkerIncreaseRateLimit = 10 * time.Millisecond
	configuration.WorkerDecreaseRateLimit = 10 * time.Millisecond
	configuration.MaxWorkers = 24
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	if maxWorkers := c.(*realComponent).config.MaxWorkers; maxWorkers != 4 {
		t.Errorf("Start() max workers should have been capped to 4 instead of %d", maxWorkers)
	}
	msg := atomic.Uint32{}
	c.StartWorkers(func(_ int, ch chan<- ScaleRequest) (ReceiveFunc, ShutdownFunc) {
		return func(context.Context, []byte) error {
			c := msg.Add(1)
			if c <= 1 {
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
	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "worker", "max", "min")
	expected := map[string]string{
		"worker_decrease_total": "0",
		"worker_increase_total": "1",
		"workers":               "1",
		"min_workers":           "1",
		"max_workers":           "4",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	t.Log("Send 1 message (increase)")
	record := &kgo.Record{
		Topic: expectedTopicName,
		Value: []byte("hello"),
	}
	if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
		t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
	}

	var diff string

	t.Log("Check if workers increased to 3")
	for range 100 {
		time.Sleep(10 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "worker")
		expected = map[string]string{
			"worker_decrease_total": "0",
			"worker_increase_total": "3",
			"workers":               "3",
		}
		if diff = helpers.Diff(gotMetrics, expected); diff == "" {
			break
		}
	}
	if diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	time.Sleep(100 * time.Millisecond)
	t.Log("Send 1 message (decrease)")
	record = &kgo.Record{
		Topic: expectedTopicName,
		Value: []byte("hello"),
	}
	if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
		t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
	}

	t.Log("Check if workers decreased to 2")
	for range 200 {
		time.Sleep(10 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "worker")
		expected = map[string]string{
			"worker_decrease_total": "1",
			"worker_increase_total": "3",
			"workers":               "2",
		}
		if diff = helpers.Diff(gotMetrics, expected); diff == "" {
			break
		}
	}
	if diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestKafkaLagMetric(t *testing.T) {
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-topic2-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(16, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	defer cluster.Close()

	// Watch for autocommits to avoid relying on time
	clusterCommitNotification := make(chan any)
	firstFetch := make(chan any)
	var firstFetchOnce sync.Once
	cluster.Control(func(request kmsg.Request) (kmsg.Response, error, bool) {
		switch k := kmsg.Key(request.Key()); k {
		case kmsg.OffsetCommit:
			t.Log("offset commit message")
			clusterCommitNotification <- nil
		case kmsg.Fetch:
			firstFetchOnce.Do(func() {
				close(firstFetch)
				t.Log("fetch request")
			})
		}
		cluster.KeepControl()
		return nil, nil, false
	})

	// Create a producer client
	producerConfiguration := kafka.DefaultConfiguration()
	producerConfiguration.Brokers = cluster.ListenAddrs()
	producerOpts, err := kafka.NewConfig(reporter.NewMock(t), producerConfiguration)
	if err != nil {
		t.Fatalf("NewConfig() error:\n%+v", err)
	}
	producerOpts = append(producerOpts, kgo.ProducerLinger(0))
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
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Start a worker with a callback that blocks on a channel after receiving a message
	workerBlockReceive := make(chan any)
	defer close(workerBlockReceive)
	c.StartWorkers(func(_ int, _ chan<- ScaleRequest) (ReceiveFunc, ShutdownFunc) {
		return func(context.Context, []byte) error {
			t.Log("worker received a message")
			<-workerBlockReceive
			return nil
		}, func() {}
	})
	t.Log("wait first fetch")
	select {
	case <-firstFetch:
	case <-time.After(time.Second):
		t.Fatal("no initial fetch")
	}

	// No messages yet, no lag
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for {
		gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "consumergroup", "workers")
		expected := map[string]string{
			"consumergroup_lag_messages": "0",
			"workers":                    "1",
		}
		if diff := helpers.Diff(gotMetrics, expected); diff != "" {
			select {
			case <-ctx.Done():
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
	}

	t.Log("send a single message")
	record := &kgo.Record{
		Topic: expectedTopicName,
		Value: []byte("hello"),
	}
	if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
		t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
	}
	t.Log("allow message processing")
	select {
	case workerBlockReceive <- nil:
	case <-time.After(time.Second):
		t.Fatal("no initial receive")
	}

	t.Log("wait for autocommit")
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for autocommit")
	case <-clusterCommitNotification:
	}

	// The message was processed, there's no lag
	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "consumergroup", "received_messages_total")
	expected := map[string]string{
		"consumergroup_lag_messages":          "0",
		`received_messages_total{worker="0"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Send a few more messages without allowing the worker to process them,
	// expect the consumer lag to rise
	t.Log("send 5 messages")
	for range 5 {
		record := &kgo.Record{
			Topic: expectedTopicName,
			Value: []byte("hello"),
		}
		if results := producer.ProduceSync(context.Background(), record); results.FirstErr() != nil {
			t.Fatalf("ProduceSync() error:\n%+v", results.FirstErr())
		}
	}

	time.Sleep(20 * time.Millisecond)
	gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "consumergroup", "received_messages_total")
	expected = map[string]string{
		"consumergroup_lag_messages":          "5",
		`received_messages_total{worker="0"}`: "2", // The consumer only blocks after incrementing the message counter
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Let the worker process all 5 messages (and wait for autocommit), expect
	// the lag to drop back to zero
	t.Log("accept processing of 5 messages")
	for range 5 {
		select {
		case workerBlockReceive <- nil:
		case <-time.After(time.Second):
			t.Fatal("no subsequent receive")
		}
	}
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for autocommit")
	case <-clusterCommitNotification:
	}
	gotMetrics = r.GetMetrics("akvorado_outlet_kafka_", "consumergroup", "received_messages_total")
	expected = map[string]string{
		"consumergroup_lag_messages":          "0",
		`received_messages_total{worker="0"}`: "6",
	}
	if diff := helpers.Diff(gotMetrics, expected); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
