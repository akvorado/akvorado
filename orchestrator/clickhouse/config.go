package clickhouse

import (
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/kafka"
)

// Configuration describes the configuration for the ClickHouse configurator.
type Configuration struct {
	clickhousedb.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Kafka describes Kafka-specific configuration
	Kafka KafkaConfiguration
	// TTL is how long to keep data. A value of 0 means to not touch TTL.
	TTL time.Duration
	// OrchestratorURL allows one to override URL to reach orchestrator from Clickhouse
	OrchestratorURL string
}

// KafkaConfiguration describes Kafka-specific configuration
type KafkaConfiguration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Consumers tell how many consumers to use to poll data from Kafka
	Consumers int
}

// DefaultConfiguration represents the default configuration for the ClickHouse configurator.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration: clickhousedb.DefaultConfiguration(),
		Kafka: KafkaConfiguration{
			Consumers: 1,
		},
	}
}
