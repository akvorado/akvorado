package clickhouse

import (
	"akvorado/common/clickhouse"
	"akvorado/common/kafka"
)

// Configuration describes the configuration for the ClickHouse configurator.
type Configuration struct {
	clickhouse.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Kafka describes Kafka-specific configuration
	Kafka KafkaConfiguration
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
		Configuration: clickhouse.DefaultConfiguration(),
		Kafka: KafkaConfiguration{
			Consumers: 1,
		},
	}
}
