// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

func TestKafka(t *testing.T) {
	r := reporter.NewMock(t)
	topic := fmt.Sprintf("flows-v%d", pb.Version)
	config := DefaultConfiguration()
	config.QueueSize = 1
	c, mock := NewMock(t, r, config)
	defer mock.Close()

	// Inject an error on third message.
	var count atomic.Uint32
	mock.ControlKey(0, func(kreq kmsg.Request) (kmsg.Response, error, bool) {
		mock.KeepControl()
		current := count.Add(1)
		if current != 3 {
			t.Logf("ControlKey() message %d: ok", current)
			return nil, nil, false
		}
		t.Logf("ControlKey() mesage %d: error", current)
		req := kreq.(*kmsg.ProduceRequest)
		resp := kreq.ResponseKind().(*kmsg.ProduceResponse)
		for _, rt := range req.Topics {
			st := kmsg.NewProduceResponseTopic()
			st.Topic = rt.Topic
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

	// Send messages
	var wg sync.WaitGroup
	wg.Add(4)
	c.Send("127.0.0.1", []byte("hello world!"), func() { wg.Done() })
	c.Send("127.0.0.1", []byte("goodbye world!"), func() { wg.Done() })
	c.Send("127.0.0.1", []byte("nooooo!"), func() { wg.Done() })
	c.Send("127.0.0.1", []byte("all good"), func() { wg.Done() })
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

	expectedMessages := []string{"hello world!", "goodbye world!", "all good"}

	// Create consumer to check messages
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(mock.ListenAddrs()...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.WithLogger(kafka.NewLogger(r)),
	)
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	defer consumer.Close()

	// Consume messages
	messages := make([]string, 0)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for {
		if len(messages) >= len(expectedMessages) {
			break
		}
		fetches := consumer.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("PollFetches() error:\n%+v", errs)
		}

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			t.Logf("EachPartition() partition %d", p.Partition)
			for _, record := range p.Records {
				messages = append(messages, string(record.Value))
				t.Logf("EachPartition() messages: %v", messages)
			}
		})
	}

	slices.Sort(expectedMessages)
	slices.Sort(messages)
	if diff := helpers.Diff(messages, expectedMessages); diff != "" {
		t.Fatalf("Send() (-got, +want):\n%s", diff)
	}

	gotMetrics := r.GetMetrics("akvorado_inlet_kafka_", "sent_", "errors")
	expectedMetrics := map[string]string{
		`sent_bytes_total{exporter="127.0.0.1"}`:    "34",
		`sent_messages_total{exporter="127.0.0.1"}`: "3",
		`errors_total{error="CORRUPT_MESSAGE"}`:     "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestLoadBalancingAlgorithm(t *testing.T) {
	for _, algo := range []LoadBalanceAlgorithm{LoadBalanceRandom, LoadBalanceByExporter} {
		t.Run(algo.String(), func(t *testing.T) {
			topic := fmt.Sprintf("balance-%s", algo)
			r := reporter.NewMock(t)
			config := DefaultConfiguration()
			config.QueueSize = 1
			config.Topic = topic
			config.LoadBalance = algo
			c, mock := NewMock(t, r, config)
			defer mock.Close()

			total := 500

			// Intercept messages
			var wg sync.WaitGroup
			wg.Add(total)
			var mu sync.Mutex
			messages := make(map[int32]int)
			kafka.InterceptMessages(t, mock, func(r *kgo.Record) {
				mu.Lock()
				defer mu.Unlock()
				messages[r.Partition]++
				wg.Done()
			})

			// Send messages
			for i := range total {
				c.Send("127.0.0.1", fmt.Appendf(nil, "hello %d", i), func() {})
			}
			wg.Wait()

			expected := make(map[int32]int, DefaultMockKafkaNumPartitions)
			switch algo {
			case LoadBalanceRandom:
				for p := range DefaultMockKafkaNumPartitions {
					p := int32(p)
					if messages[p] > total/DefaultMockKafkaNumPartitions*2/10 {
						expected[p] = messages[p]
					} else {
						expected[p] = total / DefaultMockKafkaNumPartitions
					}
				}
			case LoadBalanceByExporter:
				for p := range messages {
					expected[p] = total
					break
				}
			}

			if diff := helpers.Diff(messages, expected); diff != "" {
				t.Fatalf("Messages per partitions (-got, +want):\n%s", diff)
			}
		})
	}
}
