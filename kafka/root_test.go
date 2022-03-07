package kafka

import (
	"errors"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	gometrics "github.com/rcrowley/go-metrics"

	"flowexporter/daemon"
	"flowexporter/helpers"
	"flowexporter/reporter"
)

func TestKafka(t *testing.T) {
	r := reporter.NewMock(t)
	c, mockProducer := NewMock(t, r, DefaultConfiguration)

	// Send one message
	mockProducer.ExpectInputWithMessageCheckerFunctionAndSucceed(func(got *sarama.ProducerMessage) error {
		expected := sarama.ProducerMessage{
			Topic:     "flows",
			Key:       sarama.StringEncoder("127.0.0.1"),
			Value:     sarama.ByteEncoder("hello world!"),
			Partition: 30,
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Send() (-got, +want):\n%s", diff)
		}
		return nil
	})
	c.Send("127.0.0.1", []byte("hello world!"))

	// Another but with a fail
	mockProducer.ExpectInputAndFail(errors.New("noooo"))
	c.Send("127.0.0.1", []byte("goodbye world!"))

	time.Sleep(10 * time.Millisecond)
	gotMetrics := r.GetMetrics("flowexporter_kafka_")
	expectedMetrics := map[string]string{
		`bytes_sent{host="127.0.0.1"}`:                                           "26",
		`errors{error="kafka: Failed to produce message to topic flows: noooo"}`: "1",
		`messages_sent{host="127.0.0.1"}`:                                        "2",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}
}

func TestKafkaMetrics(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Manually put some metrics
	gometrics.GetOrRegisterMeter("incoming-byte-rate-for-broker-1111", c.kafkaConfig.MetricRegistry).
		Mark(100)
	gometrics.GetOrRegisterMeter("incoming-byte-rate-for-broker-1112", c.kafkaConfig.MetricRegistry).
		Mark(200)
	gometrics.GetOrRegisterMeter("outgoing-byte-rate-for-broker-1111", c.kafkaConfig.MetricRegistry).
		Mark(199)
	gometrics.GetOrRegisterMeter("outgoing-byte-rate-for-broker-1112", c.kafkaConfig.MetricRegistry).
		Mark(20)
	gometrics.GetOrRegisterHistogram("request-size-for-broker-1111", c.kafkaConfig.MetricRegistry,
		gometrics.NewExpDecaySample(10, 1)).
		Update(100)
	gometrics.GetOrRegisterCounter("requests-in-flight-for-broker-1111", c.kafkaConfig.MetricRegistry).
		Inc(20)
	gometrics.GetOrRegisterCounter("requests-in-flight-for-broker-1112", c.kafkaConfig.MetricRegistry).
		Inc(20)

	gotMetrics := r.GetMetrics("flowexporter_kafka_")
	expectedMetrics := map[string]string{
		`brokers_incoming_byte_rate{broker="1111"}`:            "0",
		`brokers_incoming_byte_rate{broker="1112"}`:            "0",
		`brokers_outgoing_byte_rate{broker="1111"}`:            "0",
		`brokers_outgoing_byte_rate{broker="1112"}`:            "0",
		`brokers_request_size_bucket{broker="1111",le="+Inf"}`: "1",
		`brokers_request_size_bucket{broker="1111",le="0.5"}`:  "100",
		`brokers_request_size_bucket{broker="1111",le="0.9"}`:  "100",
		`brokers_request_size_bucket{broker="1111",le="0.99"}`: "100",
		`brokers_request_size_count{broker="1111"}`:            "1",
		`brokers_request_size_sum{broker="1111"}`:              "100",
		`brokers_requests_in_flight{broker="1111"}`:            "20",
		`brokers_requests_in_flight{broker="1112"}`:            "20",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
