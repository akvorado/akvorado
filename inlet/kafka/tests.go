// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package kafka

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kfake"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// NewMock creates a new Kafka component with a mocked Kafka. It will
// panic if it cannot be started.
func NewMock(t *testing.T, reporter *reporter.Reporter, configuration Configuration) (*Component, *kfake.Cluster) {
	t.Helper()
	// Use a fake Kafka cluster for testing
	cluster, err := kfake.NewCluster(
		kfake.NumBrokers(1),
		kfake.AllowAutoTopicCreation(),
	)
	if err != nil {
		t.Fatalf("NewCluster() error: %v", err)
	}
	t.Cleanup(func() {
		cluster.Close()
	})
	configuration.Brokers = cluster.ListenAddrs()

	c, err := New(reporter, configuration, Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c, cluster
}

// Flush force flushing the currently buffered records.
func (c *Component) Flush(t *testing.T) {
	if err := c.kafkaClient.Flush(t.Context()); err != nil {
		t.Fatalf("Flush() error:\n%+v", err)
	}
}
