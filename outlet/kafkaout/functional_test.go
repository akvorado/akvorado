// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaout

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// TestFakeKafka exercises the enabled path end to end against an in-process
// fake broker: New builds the client, Start spins up the drain goroutine, Send
// enqueues, and the records are produced to (and consumed back from) the
// schema-suffixed topic. Stop is run by helpers.StartStop on cleanup.
func TestFakeKafka(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-%s", topicName, sch.ProtobufMessageHash())

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
	configuration.Enabled = true
	configuration.Topic = topicName
	configuration.Brokers = cluster.ListenAddrs()
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t), Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if !c.Enabled() {
		t.Fatal("Enabled() == false, expected true")
	}
	if c.kafkaTopic != expectedTopicName {
		t.Fatalf("topic: got %q, want %q", c.kafkaTopic, expectedTopicName)
	}
	helpers.StartStop(t, c)

	msg1 := []byte("enriched-flow-1")
	msg2 := []byte("enriched-flow-2")
	c.Send("127.0.0.1", msg1)
	c.Send("127.0.0.1", msg2)

	// The drain goroutine produces asynchronously; the send metric is bumped in
	// the produce callback once the broker acks.
	expectedMetrics := map[string]string{
		`sent_bytes_total`:    fmt.Sprintf("%d", len(msg1)+len(msg2)),
		`sent_messages_total`: "2",
	}
	metricsCtx, metricsCancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer metricsCancel()
	for {
		gotMetrics := r.GetMetrics("akvorado_outlet_kafkaout_", "sent_")
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			select {
			case <-metricsCtx.Done():
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
	}

	// Consume the two messages back to confirm they landed on the topic.
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
			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			fetches := consumer.PollFetches(ctx)
			cancel()
			if errs := fetches.Errors(); len(errs) > 0 {
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
		t.Fatalf("Didn't receive the expected messages (-got, +want):\n%s", diff)
	}
}

// TestProduceError checks the produce error path: a broker-side error is
// counted (by kerr message) and never retried, so the record is dropped rather
// than blocking the drain goroutine.
func TestProduceError(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-%s", topicName, sch.ProtobufMessageHash())

	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(1, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	defer cluster.Close()

	// Fail every produce (key 0) with a non-retriable error so the callback runs.
	cluster.ControlKey(0, func(kreq kmsg.Request) (kmsg.Response, error, bool) {
		cluster.KeepControl()
		req := kreq.(*kmsg.ProduceRequest)
		resp := kreq.ResponseKind().(*kmsg.ProduceResponse)
		for _, rt := range req.Topics {
			st := kmsg.NewProduceResponseTopic()
			st.Topic = rt.Topic
			st.TopicID = rt.TopicID
			for _, rp := range rt.Partitions {
				sp := kmsg.NewProduceResponseTopicPartition()
				sp.Partition = rp.Partition
				sp.ErrorCode = kerr.CorruptMessage.Code
				st.Partitions = append(st.Partitions, sp)
			}
			resp.Topics = append(resp.Topics, st)
		}
		return resp, nil, true
	})

	configuration := DefaultConfiguration()
	configuration.Enabled = true
	configuration.Topic = topicName
	configuration.Brokers = cluster.ListenAddrs()
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t), Schema: sch})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	c.Send("127.0.0.1", []byte("enriched-flow"))

	expectedMetrics := map[string]string{
		fmt.Sprintf(`errors_total{error="%s"}`, kerr.CorruptMessage.Message): "1",
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	for {
		gotMetrics := r.GetMetrics("akvorado_outlet_kafkaout_", "errors_")
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
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
}
