package kafka

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	gometrics "github.com/rcrowley/go-metrics"

	"akvorado/daemon"
	"akvorado/helpers"
	"akvorado/reporter"
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
	gotMetrics := r.GetMetrics("akvorado_kafka_")
	expectedMetrics := map[string]string{
		`sent_bytes_total{sampler="127.0.0.1"}`:                                        "26",
		`errors_total{error="kafka: Failed to produce message to topic flows: noooo"}`: "1",
		`sent_messages_total{sampler="127.0.0.1"}`:                                     "2",
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

	gotMetrics := r.GetMetrics("akvorado_kafka_")
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
		`brokers_inflight_requests{broker="1111"}`:             "20",
		`brokers_inflight_requests{broker="1112"}`:             "20",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestRealKafka(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip test with real Kafka in short mode")
	}

	// Kafka can either be listening right now on localhost or be
	// exposed over the hostname "kafka".
	kafkaHost := "kafka"
	mandatory := os.Getenv("AKVORADO_FUNCTIONAL_TESTS") != ""

	resolv := net.Resolver{PreferGo: true}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	_, err := resolv.LookupHost(ctx, kafkaHost)
	if err != nil {
		kafkaHost = "localhost"
	}
	cancel()

	broker := fmt.Sprintf("%s:9092", kafkaHost)
	var d net.Dialer
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	for {
		_, err := d.DialContext(ctx, "tcp", broker)
		if err == nil {
			break
		}
		if mandatory {
			t.Logf("DialContext() error:\n%+v", err)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if mandatory {
				t.Fatalf("Kafka is not running (AKVORADO_FUNCTIONAL_TESTS is set)")
			} else {
				t.Skipf("Kafka is not running (AKVORADO_FUNCTIONAL_TESTS is not set)")
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()

	// Wait for broker to be ready
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_8_1_0
	saramaConfig.Net.DialTimeout = 1 * time.Second
	saramaConfig.Net.ReadTimeout = 1 * time.Second
	saramaConfig.Net.WriteTimeout = 1 * time.Second
	ready := false
	for i := 0; i < 90; i++ {
		client, err := sarama.NewClient([]string{broker}, saramaConfig)
		if err != nil {
			continue
		}
		if err := client.RefreshMetadata(); err != nil {
			continue
		}
		brokers := client.Brokers()
		if len(brokers) == 0 {
			continue
		}
		if err := brokers[0].Open(client.Config()); err != nil {
			continue
		}
		if connected, err := brokers[0].Connected(); err != nil || !connected {
			continue
		}
		ready = true
	}
	if !ready {
		t.Fatalf("broker is not ready")
	}

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	configuration := DefaultConfiguration
	configuration.Topic = topicName
	configuration.AutoCreateTopic = true
	configuration.Brokers = []string{broker}
	configuration.Version = Version(saramaConfig.Version)
	configuration.FlushInterval = 100 * time.Millisecond
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	defer func() {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	c.Send("127.0.0.1", []byte("hello world!"))
	c.Send("127.0.0.1", []byte("goodbye world!"))

	time.Sleep(10 * time.Millisecond)
	gotMetrics := r.GetMetrics("akvorado_kafka_", "sent_")
	expectedMetrics := map[string]string{
		`sent_bytes_total{sampler="127.0.0.1"}`:    "26",
		`sent_messages_total{sampler="127.0.0.1"}`: "2",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Try to consume the two messages
	consumer, err := sarama.NewConsumer([]string{broker}, saramaConfig)
	if err != nil {
		t.Fatalf("NewConsumerGroup() error:\n%+v", err)
	}
	defer consumer.Close()
	var partitions []int32
	for {
		partitions, err = consumer.Partitions(topicName)
		if err != nil {
			if errors.Is(err, sarama.ErrUnknownTopicOrPartition) {
				// Wait for topic to be available
				continue
			}
			t.Fatalf("Partitions() error:\n%+v", err)
		}
		break
	}
	partitionConsumer, err := consumer.ConsumePartition(topicName, partitions[0], sarama.OffsetOldest)
	if err != nil {
		t.Fatalf("ConsumePartitions() error:\n%+v", err)
	}

	got := []string{}
	expected := []string{
		"127.0.0.1:hello world!",
		"127.0.0.1:goodbye world!",
	}
	timeout := time.After(15 * time.Second)
	for i := 0; i < len(expected); i++ {
		select {
		case msg := <-partitionConsumer.Messages():
			got = append(got, fmt.Sprintf("%s:%s", string(msg.Key), string(msg.Value)))
		case err := <-partitionConsumer.Errors():
			t.Fatalf("consumer.Errors():\n%+v", err)
		case <-timeout:
		}
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}
}
