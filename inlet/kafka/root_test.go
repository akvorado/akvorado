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
			t.Logf("message %d: ok", current)
			return nil, nil, false
		}
		t.Logf("mesage %d: error", current)
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
			for _, record := range p.Records {
				messages = append(messages, string(record.Value))
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
