package kafka

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"

	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	kafka.Configuration `mapstructure:",squash" yaml:",inline"`
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
	// QueueSize defines the size of the channel used to send to Kafka.
	QueueSize int
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:    kafka.DefaultConfiguration(),
		FlushInterval:    10 * time.Second,
		FlushBytes:       int(sarama.MaxRequestSize) - 1,
		MaxMessageBytes:  1000000,
		CompressionCodec: CompressionCodec(sarama.CompressionNone),
		QueueSize:        32,
	}
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
