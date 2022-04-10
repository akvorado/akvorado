package clickhouse

import "akvorado/common/kafka"

// Configuration describes the configuration for the ClickHouse configurator.
type Configuration struct {
	// Servers define the list of clickhouse servers to connect to (with ports)
	Servers []string
	// Database defines the database to use
	Database string
	// Username defines the username to use for authentication
	Username string
	// Password defines the password to use for authentication
	Password string
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
		Servers:  []string{}, // No clickhouse by default
		Database: "default",
		Username: "default",
		Kafka: KafkaConfiguration{
			Consumers: 1,
		},
	}
}
