//go:build !release

package kafka

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"

	"flowexporter/daemon"
	"flowexporter/reporter"
)

// NewMock creates a new Kafka component with a mocked Kafka. It will
// panic if it cannot be started.
func NewMock(t *testing.T, reporter *reporter.Reporter, configuration Configuration) (*Component, *mocks.AsyncProducer) {
	c, err := New(reporter, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Use a mocked Kafka producer
	var mockProducer *mocks.AsyncProducer
	c.createKafkaProducer = func() (sarama.AsyncProducer, error) {
		mockProducer = mocks.NewAsyncProducer(t, c.kafkaConfig)
		return mockProducer, nil
	}

	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}

	return c, mockProducer
}
