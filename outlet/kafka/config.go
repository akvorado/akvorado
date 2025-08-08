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
	// ConsumerGroup is the name of the consumer group to use
	ConsumerGroup string `validate:"min=1,ascii"`
	// FetchMinBytes is the minimum number of bytes to wait before fetching a message.
	FetchMinBytes int32 `validate:"min=1"`
	// FetchMaxWaitTime is the minimum duration to wait to get at least the
	// minimum number of bytes.
	FetchMaxWaitTime time.Duration `validate:"min=100ms"`
	// MinWorkers is the minimum number of workers to read messages from Kafka.
	MinWorkers int `validate:"min=1"`
	// MaxWorkers is the maximum number of workers to read messages from Kafka.
	MaxWorkers int `validate:"gtefield=MinWorkers"`
	// WorkerIncreaseRateLimit is the duration that should elapse before increasing the number of workers
	WorkerIncreaseRateLimit time.Duration `validate:"min=10s"`
	// WorkerDecreaseRateLimit is the duration that should elapse before decreasing the number of workers
	WorkerDecreaseRateLimit time.Duration `validate:"min=10s"`
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:           kafka.DefaultConfiguration(),
		ConsumerGroup:           "akvorado-outlet",
		FetchMinBytes:           1_000_000,
		FetchMaxWaitTime:        time.Second,
		MinWorkers:              1,
		MaxWorkers:              8, // This is not good to have too many workers for a single ClickHouse table.
		WorkerIncreaseRateLimit: time.Minute,
		WorkerDecreaseRateLimit: 10 * time.Minute,
	}
}

func init() {
	helpers.RegisterMapstructureDeprecatedFields[Configuration]("MaxMessageBytes", "QueueSize", "Workers")
}
