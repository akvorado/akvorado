// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
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
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
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

	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = cluster.ListenAddrs()
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	msg1 := make([]byte, 50)
	msg2 := make([]byte, 50)
	for i := range msg1 {
		msg1[i] = letters[rand.IntN(len(letters))]
	}
	for i := range msg2 {
		msg2[i] = letters[rand.IntN(len(letters))]
	}
	var wg sync.WaitGroup
	wg.Add(2)
	c.Send("127.0.0.1", msg1, func() { wg.Done() })
	c.Send("127.0.0.1", msg2, func() { wg.Done() })
	c.Flush(t)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Send() timeout")
	}

	gotMetrics := r.GetMetrics("akvorado_inlet_kafka_", "sent_", "connects_")
	expectedMetrics := map[string]string{
		// Our own metrics
		`sent_bytes_total{exporter="127.0.0.1"}`:    "100",
		`sent_messages_total{exporter="127.0.0.1"}`: "2",
		// From franz-go
		`connects_total{node_id="0"}`:      "2",
		`connects_total{node_id="seed_0"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Try to consume the two messages using franz-go
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(cluster.ListenAddrs()...),
		kgo.ConsumeTopics(expectedTopicName),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.FetchMaxWait(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	defer consumer.Close()

	got := []string{}
	expected := []string{string(msg1), string(msg2)}
	timeout := time.After(15 * time.Second)
	for len(got) < len(expected) {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for messages. Got %d of %d messages", len(got), len(expected))
		default:
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			fetches := consumer.PollFetches(ctx)
			cancel()
			if errs := fetches.Errors(); len(errs) > 0 {
				t.Logf("PollFetches() error: %+v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				for _, record := range p.Records {
					got = append(got, string(record.Value))
				}
			})
		}
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}
}
