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
	// AkvoradoURL allows one to override URL to reach Akvorado from Clickhouse
	AkvoradoURL string
}

// KafkaConfiguration describes Kafka-specific configuration
type KafkaConfiguration struct {
	kafka.Configuration
	// Consumers tell how many consumers to use to poll data from Kafka
	Consumers int
}

// DefaultConfiguration represents the default configuration for the ClickHouse configurator.
var DefaultConfiguration = Configuration{
	Servers:  []string{}, // No clickhouse by default
	Database: "default",
	Username: "default",
	Kafka: KafkaConfiguration{
		Consumers: 1,
	},
}
