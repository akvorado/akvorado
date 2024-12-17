// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"testing"
	"time"

	gometrics "github.com/rcrowley/go-metrics"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestMock(t *testing.T) {
	c, incoming := NewMock(t, DefaultConfiguration())

	got := []string{}
	expected := []string{"hello1", "hello2", "hello3"}
	gotAll := make(chan bool)
	shutdownCalled := false
	callback := func(_ context.Context, message []byte) error {
		got = append(got, string(message))
		if len(got) == len(expected) {
			close(gotAll)
		}
		return nil
	}
	c.StartWorkers(
		func(_ int) (ReceiveFunc, ShutdownFunc) {
			return callback, func() { shutdownCalled = true }
		},
	)

	// Produce messages and wait for them
	for _, msg := range expected {
		incoming <- []byte(msg)
	}
	select {
	case <-time.After(time.Second):
		t.Fatal("Too long to get messages")
	case <-gotAll:
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}

	c.Stop()
	if !shutdownCalled {
		t.Error("Stop() should have triggered shutdown function")
	}
}

func TestKafkaMetrics(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	kafkaConfig := c.(*realComponent).kafkaConfig

	// Manually put some metrics
	gometrics.GetOrRegisterMeter("consumer-fetch-rate", kafkaConfig.MetricRegistry).
		Mark(30)
	gometrics.GetOrRegisterHistogram("consumer-batch-size", kafkaConfig.MetricRegistry,
		gometrics.NewExpDecaySample(10, 1)).
		Update(100)
	gometrics.GetOrRegisterHistogram("consumer-fetch-response-size", kafkaConfig.MetricRegistry,
		gometrics.NewExpDecaySample(10, 1)).
		Update(200)
	gometrics.GetOrRegisterCounter("consumer-group-join-total-akvorado", kafkaConfig.MetricRegistry).
		Inc(20)
	gometrics.GetOrRegisterCounter("consumer-group-join-failed-akvorado", kafkaConfig.MetricRegistry).
		Inc(1)
	gometrics.GetOrRegisterCounter("consumer-group-sync-total-akvorado", kafkaConfig.MetricRegistry).
		Inc(4)
	gometrics.GetOrRegisterCounter("consumer-group-sync-failed-akvorado", kafkaConfig.MetricRegistry).
		Inc(1)

	gotMetrics := r.GetMetrics("akvorado_outlet_kafka_", "-consumer_fetch_rate")
	expectedMetrics := map[string]string{
		`consumer_batch_messages_bucket{le="+Inf"}`:          "1",
		`consumer_batch_messages_bucket{le="0.5"}`:           "100",
		`consumer_batch_messages_bucket{le="0.9"}`:           "100",
		`consumer_batch_messages_bucket{le="0.99"}`:          "100",
		`consumer_batch_messages_count`:                      "1",
		`consumer_batch_messages_sum`:                        "100",
		`consumer_fetch_bytes_bucket{le="+Inf"}`:             "1",
		`consumer_fetch_bytes_bucket{le="0.5"}`:              "200",
		`consumer_fetch_bytes_bucket{le="0.9"}`:              "200",
		`consumer_fetch_bytes_bucket{le="0.99"}`:             "200",
		`consumer_fetch_bytes_count`:                         "1",
		`consumer_fetch_bytes_sum`:                           "200",
		`consumer_group_join_total{group="akvorado"}`:        "20",
		`consumer_group_join_failed_total{group="akvorado"}`: "1",
		`consumer_group_sync_total{group="akvorado"}`:        "4",
		`consumer_group_sync_failed_total{group="akvorado"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
