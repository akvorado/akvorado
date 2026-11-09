// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkainput

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"akvorado/common/daemon"
	"akvorado/common/kafka"
	"akvorado/common/pb"
	"akvorado/common/reporter"
)

// commitHarness runs the component against a fake Kafka cluster.
type commitHarness struct {
	t        *testing.T
	cluster  *kfake.Cluster
	producer *kgo.Client
	c        *realComponent
	topic    string
	group    string
}

func newCommitHarness(t *testing.T, partitions int32) *commitHarness {
	t.Helper()
	r := reporter.NewMock(t)
	topicName := fmt.Sprintf("test-commit-%d", rand.Int())
	expectedTopicName := fmt.Sprintf("%s-v%d", topicName, pb.Version)
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.SeedTopics(partitions, expectedTopicName),
		kfake.WithLogger(kafka.NewLogger(r)),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	t.Cleanup(cluster.Close)

	producerConfiguration := kafka.DefaultConfiguration()
	producerConfiguration.Brokers = cluster.ListenAddrs()
	producerOpts, err := kafka.NewConfig(reporter.NewMock(t), producerConfiguration)
	if err != nil {
		t.Fatalf("NewConfig() error:\n%+v", err)
	}
	producerOpts = append(producerOpts,
		kgo.ProducerLinger(0),
		kgo.RecordPartitioner(kgo.ManualPartitioner()))
	producer, err := kgo.NewClient(producerOpts...)
	if err != nil {
		t.Fatalf("NewClient() error:\n%+v", err)
	}
	t.Cleanup(producer.Close)

	configuration := DefaultConfiguration()
	configuration.Topic = topicName
	configuration.Brokers = cluster.ListenAddrs()
	configuration.FetchMaxWaitTime = 10 * time.Millisecond
	configuration.ConsumerGroup = fmt.Sprintf("outlet-%d", rand.Int())
	configuration.MaxWorkers = int(partitions)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.(*realComponent).Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	t.Cleanup(func() { c.Stop() })

	return &commitHarness{
		t:        t,
		cluster:  cluster,
		producer: producer,
		c:        c.(*realComponent),
		topic:    expectedTopicName,
		group:    configuration.ConsumerGroup,
	}
}

// produce sends one record per value to the given partition.
func (h *commitHarness) produce(partition int32, values ...string) {
	h.t.Helper()
	for _, value := range values {
		record := &kgo.Record{Topic: h.topic, Partition: partition, Value: []byte(value)}
		if err := h.producer.ProduceSync(h.t.Context(), record).FirstErr(); err != nil {
			h.t.Fatalf("ProduceSync() error:\n%+v", err)
		}
	}
}

// waitFetching waits for the first fetch of the workers.
func (h *commitHarness) waitFetching() {
	h.t.Helper()
	fetchFault := h.cluster.Fault(kfake.Fault{Keys: []kmsg.Key{kmsg.Fetch}, Observe: true, Count: -1})
	defer fetchFault.Remove()
	ctx, cancel := context.WithTimeout(h.t.Context(), 5*time.Second)
	defer cancel()
	if err := fetchFault.Wait(ctx, 1); err != nil {
		h.t.Fatalf("Wait() error:\n%+v", err)
	}
}

// committed returns the sum of committed offsets of the group.
func (h *commitHarness) committed() int64 {
	var total int64
	if g := h.cluster.GroupInfo(h.group); g != nil {
		for _, commit := range g.Commits[h.topic] {
			total += commit.Offset
		}
	}
	return total
}

// waitCommitted waits for the sum of committed offsets to reach expected.
func (h *commitHarness) waitCommitted(expected int64) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h.committed() == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("committed offsets: got %d, want %d", h.committed(), expected)
}

// assertCommittedStays checks the committed offsets stay at expected for
// longer than the former autocommit interval.
func (h *commitHarness) assertCommittedStays(expected int64) {
	h.t.Helper()
	time.Sleep(1500 * time.Millisecond)
	if got := h.committed(); got != expected {
		h.t.Fatalf("committed offsets: got %d, want %d", got, expected)
	}
}

// recorder records received messages and answers with a scripted status.
type recorder struct {
	mu       sync.Mutex
	received []string
	statuses map[string]Flushed
	notify   chan string
}

func newRecorder(statuses map[string]Flushed) *recorder {
	return &recorder{statuses: statuses, notify: make(chan string, 100)}
}

func (r *recorder) receive(_ context.Context, data []byte) (Flushed, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = append(r.received, string(data))
	r.notify <- string(data)
	return r.statuses[string(data)], nil
}

func (r *recorder) wait(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-r.notify:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not receive a message")
		}
	}
}

func TestCommitOnlyAfterFlush(t *testing.T) {
	h := newCommitHarness(t, 1)
	rec := newRecorder(map[string]Flushed{"d": FlushedAll})
	h.c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, noFlush, func() {}
	})
	h.waitFetching()

	// Records in the batch, no flush: a crash here must replay them.
	h.produce(0, "a", "b", "c")
	rec.wait(t, 3)
	h.assertCommittedStays(0)

	// The flush includes all records.
	h.produce(0, "d")
	rec.wait(t, 1)
	h.waitCommitted(4)
}

func TestCommitPartialRecord(t *testing.T) {
	h := newCommitHarness(t, 1)
	rec := newRecorder(map[string]Flushed{"c": FlushedBefore})
	h.c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, noFlush, func() {}
	})
	h.waitFetching()

	// "c" is partly flushed: commit up to "c" excluded.
	h.produce(0, "a", "b", "c")
	rec.wait(t, 3)
	h.waitCommitted(2)
	h.assertCommittedStays(2)
}

func TestStopFlushesThenCommits(t *testing.T) {
	h := newCommitHarness(t, 1)
	rec := newRecorder(nil)
	var mu sync.Mutex
	steps := []string{}
	step := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		steps = append(steps, s)
	}
	h.c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, func(context.Context) error {
				step(fmt.Sprintf("flush (committed=%d)", h.committed()))
				return nil
			}, func() {
				step(fmt.Sprintf("shutdown (committed=%d)", h.committed()))
			}
	})
	h.waitFetching()
	h.produce(0, "a", "b", "c")
	rec.wait(t, 3)

	h.c.StopWorkers()
	h.waitCommitted(3)
	mu.Lock()
	defer mu.Unlock()
	expected := []string{"flush (committed=0)", "shutdown (committed=3)"}
	if fmt.Sprint(steps) != fmt.Sprint(expected) {
		t.Fatalf("StopWorkers() steps: got %q, want %q", steps, expected)
	}
}

func TestStopFailedFlushCommitsNothing(t *testing.T) {
	h := newCommitHarness(t, 1)
	rec := newRecorder(nil)
	shutdown := make(chan struct{})
	h.c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, func(context.Context) error {
			return errors.New("ClickHouse unavailable")
		}, func() { close(shutdown) }
	})
	h.waitFetching()
	h.produce(0, "a", "b", "c")
	rec.wait(t, 3)

	h.c.StopWorkers()
	<-shutdown
	if got := h.committed(); got != 0 {
		t.Fatalf("committed offsets after failed flush: got %d, want 0", got)
	}
}

func TestRevokeFlushesThenCommits(t *testing.T) {
	h := newCommitHarness(t, 2)
	rec := newRecorder(nil)
	var mu sync.Mutex
	flushes := map[int]int{}
	h.c.StartWorkers(func(i int, _ chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			flushes[i]++
			return nil
		}, func() {}
	})
	h.waitFetching()

	// The single worker owns both partitions.
	h.produce(0, "a", "b")
	h.produce(1, "c")
	rec.wait(t, 3)

	// A new worker takes one partition: the first one flushes and commits
	// before it leaves.
	if err := h.c.startOneWorker(); err != nil {
		t.Fatalf("startOneWorker() error:\n%+v", err)
	}
	h.waitCommitted(3)
	mu.Lock()
	defer mu.Unlock()
	if flushes[0] == 0 {
		t.Fatal("revoke did not flush worker 0")
	}
}
