// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaout

import (
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

// TestTopicSchemaSuffix checks the topic always gets the schema hash appended, so
// an incompatible schema change lands on a new topic. The component stays
// disabled so New only exercises the naming, not the Kafka client.
func TestTopicSchemaSuffix(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	deps := Dependencies{Schema: sch}

	c, err := New(r, Configuration{Configuration: kafka.Configuration{Topic: "flows-enriched"}}, deps)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	want := "flows-enriched-" + sch.ProtobufMessageHash()
	if c.kafkaTopic != want {
		t.Errorf("topic: got %q, want %q", c.kafkaTopic, want)
	}
}

// TestSendDropsWhenFull checks the load-shedding contract: when the queue is
// full, Send drops (and counts) instead of blocking the caller. No drain
// goroutine is started, so the cap-1 queue stays full after the first Send.
func TestSendDropsWhenFull(t *testing.T) {
	r := reporter.NewMock(t)
	c := &Component{
		r:           r,
		kafkaTopic:  "flows-enriched",
		kafkaClient: &kgo.Client{}, // non-nil; Send only checks != nil, never calls into it
		sendCh:      make(chan *kgo.Record, 1),
	}
	c.initMetrics()

	c.Send("k", []byte("a")) // fills the cap-1 queue
	done := make(chan struct{})
	go func() {
		c.Send("k", []byte("b")) // queue full -> drop
		c.Send("k", []byte("c")) // queue full -> drop
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Send blocked while the queue was full")
	}

	got := r.GetMetrics("akvorado_outlet_kafkaout_", "dropped_messages_total")
	expected := map[string]string{"dropped_messages_total": "2"}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("dropped metric (-got, +want):\n%s", diff)
	}
}
