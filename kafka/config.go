package kafka

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	// Topic defines the topic to write flows to.
	Topic string
	// TopicConfiguration describes the topic configuration. If none is provided, it will not be created.
	TopicConfiguration *TopicConfiguration
	// Brokers is the list of brokers to connect to.
	Brokers []string
	// Version is the version of Kafka we assume to work
	Version Version
	// UseTls tells if we should use TLS.
	UseTLS bool
	// FlushInterval tells how often to flush pending data to Kafka.
	FlushInterval time.Duration
	// FlushBytes tells to flush when there are many bytes to write
	FlushBytes int
	// MaxMessageBytes is the maximum permitted size of a message.
	// Should be set equal or smaller than broker's
	// `message.max.bytes`.
	MaxMessageBytes int
	// CompressionCodec defines the compression to use.
	CompressionCodec CompressionCodec
}

// TopicConfiguration describes the configuration for a topic
type TopicConfiguration struct {
	// NumPartitions tells how many partitions should be used for the topic.
	NumPartitions int32
	// ReplicationFactor tells the replication factor for the topic.
	ReplicationFactor int16
	// ConfigEntries is a map to specify the topic overrides. Non-listed overrides will be removed
	ConfigEntries map[string]*string
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
var DefaultConfiguration = Configuration{
	Topic:            "flows",
	Brokers:          []string{"127.0.0.1:9092"},
	Version:          Version(sarama.DefaultVersion),
	UseTLS:           false,
	FlushInterval:    10 * time.Second,
	FlushBytes:       int(sarama.MaxRequestSize) - 1,
	MaxMessageBytes:  1000000,
	CompressionCodec: CompressionCodec(sarama.CompressionNone),
}

// Version represents a supported version of Kafka
type Version sarama.KafkaVersion

// UnmarshalText parses a version of Kafka
func (v *Version) UnmarshalText(text []byte) error {
	version, err := sarama.ParseKafkaVersion(string(text))
	if err != nil {
		return err
	}
	*v = Version(version)
	return nil
}

// String turns a Kafka version into a string
func (v Version) String() string {
	return sarama.KafkaVersion(v).String()
}

// MarshalText turns a Kafka version intro a string
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// CompressionCodec represents a compression codec.
type CompressionCodec sarama.CompressionCodec

// UnmarshalText produces a compression codec
func (cc *CompressionCodec) UnmarshalText(text []byte) error {
	codecs := map[string]sarama.CompressionCodec{
		"none":   sarama.CompressionNone,
		"gzip":   sarama.CompressionGZIP,
		"snappy": sarama.CompressionSnappy,
		"lz4":    sarama.CompressionLZ4,
		"zstd":   sarama.CompressionZSTD,
	}
	codec, ok := codecs[string(text)]
	if !ok {
		return fmt.Errorf("cannot parse %q as a compression codec", string(text))
	}
	*cc = CompressionCodec(codec)
	return nil
}

// String turns a compression codec into a string
func (cc CompressionCodec) String() string {
	return sarama.CompressionCodec(cc).String()
}

// MarshalText turns a compression codec into a string
func (cc CompressionCodec) MarshalText() ([]byte, error) {
	return []byte(cc.String()), nil
}
