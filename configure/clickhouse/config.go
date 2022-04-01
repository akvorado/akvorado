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
	// Kafka describes how to connect to Kafka
	Kafka kafka.Configuration `yaml:"-"`
	// KafkaThreads tell how many threads to use to poll data from Kafka
	KafkaThreads int
	// AkvoradoURL allows one to override URL to reach Akvorado from Clickhouse
	AkvoradoURL string
}

// DefaultConfiguration represents the default configuration for the ClickHouse configurator.
var DefaultConfiguration = Configuration{
	Servers:      []string{}, // No clickhouse by default
	Database:     "default",
	Username:     "default",
	Kafka:        kafka.DefaultConfiguration,
	KafkaThreads: 1,
}
