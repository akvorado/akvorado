// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"time"

	"github.com/IBM/sarama"

	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// FlushInterval tells how often to flush pending data to Kafka.
	FlushInterval time.Duration `validate:"min=100ms"`
	// FlushBytes tells to flush when there are many bytes to write
	FlushBytes int `validate:"min=1000"`
	// MaxMessageBytes is the maximum permitted size of a message.
	// Should be set equal or smaller than broker's
	// `message.max.bytes`.
	MaxMessageBytes int `validate:"min=1"`
	// CompressionCodec defines the compression to use.
	CompressionCodec CompressionCodec
	// QueueSize defines the size of the channel used to send to Kafka.
	QueueSize int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:    kafka.DefaultConfiguration(),
		FlushInterval:    time.Second,
		FlushBytes:       int(sarama.MaxRequestSize) - 1,
		MaxMessageBytes:  1_000_000,
		CompressionCodec: CompressionCodec(sarama.CompressionNone),
		QueueSize:        32,
	}
}

// CompressionCodec represents a compression codec.
type CompressionCodec sarama.CompressionCodec

// UnmarshalText produces a compression codec
func (cc *CompressionCodec) UnmarshalText(text []byte) error {
	return (*sarama.CompressionCodec)(cc).UnmarshalText(text)
}

// String turns a compression codec into a string
func (cc CompressionCodec) String() string {
	return sarama.CompressionCodec(cc).String()
}

// MarshalText turns a compression codec into a string
func (cc CompressionCodec) MarshalText() ([]byte, error) {
	return []byte(cc.String()), nil
}
