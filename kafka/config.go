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
	// Brokers is the list of brokers to connect to.
	Brokers []string
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

// DefaultConfiguration represents the default configuration for the Kafka exporter.
var DefaultConfiguration = Configuration{
	Topic:           "flows",
	Brokers:         []string{"127.0.0.1:9092"},
	UseTLS:          false,
	FlushInterval:   10 * time.Second,
	FlushBytes:      int(sarama.MaxRequestSize),
	MaxMessageBytes: 1000000,
}

// CompressionCodec represents a compression codec.
type CompressionCodec sarama.CompressionCodec

// UnmarshalText produces a compression codec
func (c *CompressionCodec) UnmarshalText(text []byte) error {
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
	*c = CompressionCodec(codec)
	return nil
}
