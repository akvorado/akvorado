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

	"akvorado/daemon"
	"akvorado/helpers"
	"akvorado/reporter"
)

func setupKafkaBroker(t *testing.T) (sarama.Client, []string) {
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
	var client sarama.Client
	for i := 0; i < 90; i++ {
		client, err = sarama.NewClient([]string{broker}, saramaConfig)
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

	return client, []string{broker}
}

func TestRealKafka(t *testing.T) {
	client, brokers := setupKafkaBroker(t)

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	configuration := DefaultConfiguration
	configuration.Topic = topicName
	configuration.TopicConfiguration = &TopicConfiguration{
		NumPartitions:     1,
		ReplicationFactor: 1,
	}
	configuration.Brokers = brokers
	configuration.Version = Version(sarama.V2_8_1_0)
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
	consumer, err := sarama.NewConsumerFromClient(client)
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

func TestTopicCreation(t *testing.T) {
	client, brokers := setupKafkaBroker(t)

	rand.Seed(time.Now().UnixMicro())
	topicName := fmt.Sprintf("test-topic-%d", rand.Int())
	retentionMs := "76548"
	segmentBytes := "107374184"
	segmentBytes2 := "10737184"
	cleanupPolicy := "delete"

	cases := []struct {
		Name          string
		ConfigEntries map[string]*string
	}{
		{
			Name: "Set initial config",
			ConfigEntries: map[string]*string{
				"retention.ms":  &retentionMs,
				"segment.bytes": &segmentBytes,
			},
		}, {
			Name: "Alter initial config",
			ConfigEntries: map[string]*string{
				"retention.ms":   &retentionMs,
				"segment.bytes":  &segmentBytes2,
				"cleanup.policy": &cleanupPolicy,
			},
		}, {
			Name: "Remove item",
			ConfigEntries: map[string]*string{
				"retention.ms":  &retentionMs,
				"segment.bytes": &segmentBytes2,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			configuration := DefaultConfiguration
			configuration.Topic = topicName
			configuration.TopicConfiguration = &TopicConfiguration{
				NumPartitions:     1,
				ReplicationFactor: 1,
				ConfigEntries:     tc.ConfigEntries,
			}
			configuration.Brokers = brokers
			configuration.Version = Version(sarama.V2_8_1_0)
			c, err := New(reporter.NewMock(t), configuration, Dependencies{Daemon: daemon.NewMock(t)})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			if err := c.Start(); err != nil {
				t.Fatalf("Start() error:\n%+v", err)
			}
			if err := c.Stop(); err != nil {
				t.Fatalf("Start() error:\n%+v", err)
			}

			adminClient, err := sarama.NewClusterAdminFromClient(client)
			if err != nil {
				t.Fatalf("NewClusterAdmin() error:\n%+v", err)
			}
			topics, err := adminClient.ListTopics()
			if err != nil {
				t.Fatalf("ListTopics() error:\n%+v", err)
			}
			topic, ok := topics[topicName]
			if !ok {
				t.Fatal("ListTopics() did not find the topic")
			}
			if diff := helpers.Diff(topic.ConfigEntries, tc.ConfigEntries); diff != "" {
				t.Fatalf("ListTopics() (-got, +want):\n%s", diff)
			}
		})
	}

}
