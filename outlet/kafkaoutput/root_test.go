// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaoutput

import (
	"context"
	"testing"
	"time"

	"akvorado/common/daemon"
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

// TestDisabled checks the component is inert when disabled: Start/Stop are
// no-ops and Send drops on the nil client, so an existing deployment that never
// enables the output is unaffected.
func TestDisabled(t *testing.T) {
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	deps := Dependencies{Schema: sch}

	c, err := New(r, Configuration{Configuration: kafka.Configuration{Topic: "flows-enriched"}}, deps)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if c.Enabled() {
		t.Error("Enabled() == true, expected false")
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	c.Send("k", []byte("dropped")) // nil client -> no-op
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}
}

// TestSendDropsWhenFull checks the load-shedding contract: when the producer
// buffer is full, Send drops (and counts) instead of blocking the caller. The
// broker address is a black hole, so the first record stays buffered and the
// next two find no room.
func TestSendDropsWhenFull(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration()
	configuration.Enabled = true
	configuration.Brokers = []string{"127.0.0.1:1"}
	configuration.QueueSize = 1
	// The buffered record can never be flushed, so don't wait for it on stop.
	configuration.ShutdownTimeout = 0
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t), Schema: schema.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	done := make(chan struct{})
	go func() {
		c.Send("127.0.0.1", []byte("a")) // fills the one-record buffer
		c.Send("127.0.0.1", []byte("b")) // buffer full -> drop
		c.Send("127.0.0.1", []byte("c")) // buffer full -> drop
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Send blocked while the producer buffer was full")
	}

	// The produce promises run on their own goroutine, so the counter lags a bit
	// behind the calls to Send.
	expected := map[string]string{"dropped_messages_total": "2"}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	for {
		got := r.GetMetrics("akvorado_outlet_kafkaoutput_", "dropped_messages_total")
		if diff := helpers.Diff(got, expected); diff != "" {
			select {
			case <-ctx.Done():
				t.Fatalf("dropped metric (-got, +want):\n%s", diff)
			default:
			}
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
	}
}
