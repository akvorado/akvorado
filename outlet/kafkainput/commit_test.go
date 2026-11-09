// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkainput

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
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

// sumCommits returns the sum of committed offsets of a group for a topic.
func sumCommits(g *kfake.GroupInfo, topic string) int64 {
	var total int64
	if g != nil {
		for _, commit := range g.Commits[topic] {
			total += commit.Offset
		}
	}
	return total
}

// committedOffsets returns the sum of committed offsets of a group.
func committedOffsets(cluster *kfake.Cluster, group, topic string) int64 {
	return sumCommits(cluster.GroupInfo(group), topic)
}

// waitCommitted waits for the sum of committed offsets of a group to reach
// expected.
func waitCommitted(t *testing.T, cluster *kfake.Cluster, group, topic string, expected int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := cluster.WaitGroupInfo(ctx, group, func(g *kfake.GroupInfo) bool {
		return sumCommits(g, topic) == expected
	}); err != nil {
		t.Fatalf("WaitGroupInfo() error, committed %d, want %d:\n%+v",
			committedOffsets(cluster, group, topic), expected, err)
	}
}

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

func (h *commitHarness) committed() int64 {
	return committedOffsets(h.cluster, h.group, h.topic)
}

func (h *commitHarness) waitCommitted(expected int64) {
	h.t.Helper()
	waitCommitted(h.t, h.cluster, h.group, h.topic, expected)
}

// assertCommittedStays checks the committed offsets do not move: no commit
// without a flush.
func (h *commitHarness) assertCommittedStays(expected int64) {
	h.t.Helper()
	time.Sleep(100 * time.Millisecond)
	if diff := helpers.Diff(h.committed(), expected); diff != "" {
		h.t.Fatalf("committed offsets (-got, +want):\n%s", diff)
	}
}

// recorder signals received messages and reports a flush for the scripted
// ones.
type recorder struct {
	flushOn map[string]bool
	notify  chan string
}

func newRecorder(flushOn ...string) *recorder {
	r := &recorder{flushOn: map[string]bool{}, notify: make(chan string, 100)}
	for _, value := range flushOn {
		r.flushOn[value] = true
	}
	return r
}

func (r *recorder) receive(_ context.Context, data []byte) (bool, error) {
	r.notify <- string(data)
	return r.flushOn[string(data)], nil
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
	rec := newRecorder("d")
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

func TestCommitAllPartitionsOnFlush(t *testing.T) {
	h := newCommitHarness(t, 2)
	rec := newRecorder("c")
	h.c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, noFlush, func() {}
	})
	h.waitFetching()

	h.produce(0, "a", "b")
	rec.wait(t, 2)
	h.assertCommittedStays(0)

	// A flush on partition 1 also writes the records of partition 0.
	h.produce(1, "c")
	rec.wait(t, 1)
	h.waitCommitted(3)

	// The next record is not flushed yet.
	h.produce(0, "d")
	rec.wait(t, 1)
	h.assertCommittedStays(3)
}

func TestStopFlushesThenCommits(t *testing.T) {
	h := newCommitHarness(t, 1)
	rec := newRecorder()
	var atFlush, atShutdown int64 = -1, -1
	h.c.StartWorkers(func(int, chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, func(context.Context) error {
				atFlush = h.committed()
				return nil
			}, func() {
				atShutdown = h.committed()
			}
	})
	h.waitFetching()
	h.produce(0, "a", "b", "c")
	rec.wait(t, 3)

	h.c.StopWorkers()
	if diff := helpers.Diff([]int64{atFlush, atShutdown}, []int64{0, 3}); diff != "" {
		t.Fatalf("committed offsets at flush and at shutdown (-got, +want):\n%s", diff)
	}
}

func TestStopFailedFlushCommitsNothing(t *testing.T) {
	h := newCommitHarness(t, 1)
	rec := newRecorder()
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
	if diff := helpers.Diff(h.committed(), int64(0)); diff != "" {
		t.Fatalf("committed offsets after failed flush (-got, +want):\n%s", diff)
	}
}

func TestRevokeFlushesThenCommits(t *testing.T) {
	h := newCommitHarness(t, 2)
	rec := newRecorder()
	var flushed atomic.Bool
	h.c.StartWorkers(func(i int, _ chan<- ScaleRequest) (ReceiveFunc, FlushFunc, ShutdownFunc) {
		return rec.receive, func(context.Context) error {
			if i == 0 {
				flushed.Store(true)
			}
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
	if !flushed.Load() {
		t.Fatal("revoke did not flush worker 0")
	}
}
