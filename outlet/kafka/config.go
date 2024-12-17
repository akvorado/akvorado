// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"time"

	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Workers define the number of workers to read messages from Kafka.
	Workers int `validate:"min=1"`
	// ConsumerGroup is the name of the consumer group to use
	ConsumerGroup string `validate:"min=1,ascii"`
	// MaxMessageBytes is the maximum permitted size of a message. Should be set
	// equal or smaller than broker's `message.max.bytes`.
	MaxMessageBytes int32 `validate:"min=1"`
	// FetchMinBytes is the minimum number of bytes to wait before fetching a message.
	FetchMinBytes int32 `validate:"min=1"`
	// FetchMaxWaitTime is the minimum duration to wait to get at least the
	// minimum number of bytes.
	FetchMaxWaitTime time.Duration `validate:"min=100ms"`
	// QueueSize defines the size of the channel used to receive from Kafka.
	QueueSize int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:    kafka.DefaultConfiguration(),
		Workers:          1,
		ConsumerGroup:    "akvorado-outlet",
		MaxMessageBytes:  1_000_000,
		FetchMinBytes:    1_000_000,
		FetchMaxWaitTime: time.Second,
		QueueSize:        32,
	}
}
