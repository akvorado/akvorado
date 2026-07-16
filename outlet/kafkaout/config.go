// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafkaout

import (
	"akvorado/common/kafka"
)

// Configuration describes the configuration for the Kafka output (exporting
// enriched flows to a Kafka topic in parallel with ClickHouse).
type Configuration struct {
	// Enabled turns the Kafka output on. Disabled by default so existing
	// deployments are unaffected.
	Enabled             bool
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// QueueSize is the producer buffer: the max records held in flight
	// (kgo MaxBufferedRecords) and the send-queue depth. When full, records are
	// dropped, not blocked (best-effort; see dropped_messages_total and the
	// kafka-out docs for sizing).
	QueueSize int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the Kafka output.
func DefaultConfiguration() Configuration {
	cfg := kafka.DefaultConfiguration()
	cfg.Topic = "flows-enriched"
	return Configuration{
		Enabled:       false,
		Configuration: cfg,
		QueueSize:     4096,
	}
}
