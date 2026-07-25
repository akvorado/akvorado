// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaoutput

import (
	"github.com/twmb/franz-go/pkg/kgo"

	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka output (exporting
// enriched flows to a Kafka topic in parallel with ClickHouse).
type Configuration struct {
	// Enabled turns the Kafka output on. Disabled by default so existing
	// deployments are unaffected.
	Enabled             bool
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// CompressionCodec defines the compression to use.
	CompressionCodec kafka.CompressionCodec
	// QueueSize is the producer buffer: the max records held in flight
	// (kgo MaxBufferedRecords) and the send-queue depth. When full, records are
	// dropped, not blocked (best-effort; see dropped_messages_total and the
	// kafka-output docs for sizing).
	QueueSize int `validate:"min=1"`
	// LoadBalance defines the load-balancing algorithm to use for Kafka partitions.
	LoadBalance kafka.LoadBalanceAlgorithm
}

// DefaultConfiguration represents the default configuration for the Kafka output.
func DefaultConfiguration() Configuration {
	cfg := kafka.DefaultConfiguration()
	cfg.Topic = "flows-enriched"
	return Configuration{
		Enabled:          false,
		Configuration:    cfg,
		CompressionCodec: kafka.CompressionCodec(kgo.Lz4Compression()),
		QueueSize:        4096,
		LoadBalance:      kafka.LoadBalanceRandom,
	}
}
