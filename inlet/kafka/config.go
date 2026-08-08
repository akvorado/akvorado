// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/helpers"
	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka exporter.
type Configuration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// CompressionCodec defines the compression to use.
	CompressionCodec kafka.CompressionCodec
	// QueueSize defines the maximum number of messages to buffer.
	QueueSize int `validate:"min=1"`
	// LoadBalance defines the load-balancing algorithm to use for Kafka partitions.
	LoadBalance kafka.LoadBalanceAlgorithm
}

// DefaultConfiguration represents the default configuration for the Kafka exporter.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration:    kafka.DefaultConfiguration(),
		CompressionCodec: kafka.CompressionCodec(kgo.Lz4Compression()),
		QueueSize:        4096,
		LoadBalance:      kafka.LoadBalanceRandom,
	}
}

func init() {
	helpers.RegisterMapstructureDeprecatedFields[Configuration](
		"FlushInterval",   // bad for performance
		"FlushBytes",      //  duplicate with QueueSize
		"MaxMessageBytes", //  just tune QueueSize instead
	)
}
