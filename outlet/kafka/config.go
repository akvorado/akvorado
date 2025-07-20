// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"time"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Workers define the number of workers to read messages from Kafka.
	Workers int `validate:"min=1"`
	// ConsumerGroup is the name of the consumer group to use
	ConsumerGroup string `validate:"min=1,ascii"`
	// FetchMinBytes is the minimum number of bytes to wait before fetching a message.
	FetchMinBytes int32 `validate:"min=1"`
	// FetchMaxWaitTime is the minimum duration to wait to get at least the
	// minimum number of bytes.
	FetchMaxWaitTime time.Duration `validate:"min=100ms"`
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:    kafka.DefaultConfiguration(),
		Workers:          1,
		ConsumerGroup:    "akvorado-outlet",
		FetchMinBytes:    1_000_000,
		FetchMaxWaitTime: time.Second,
	}
}

func init() {
	helpers.RegisterMapstructureDeprecatedFields[Configuration]("MaxMessageBytes", "QueueSize")
}
