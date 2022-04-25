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
	// Resolutions describe the various resolutions to use to
	// store data and the associated TTLs.
	Resolutions []ResolutionConfiguration
	// OrchestratorURL allows one to override URL to reach orchestrator from Clickhouse
	OrchestratorURL string
}

// ResolutionConfiguration describes a consolidation interval.
type ResolutionConfiguration struct {
	// Interval is the consolidation interval for this
	// resolution. An interval of 0 means no consolidation
	// takes place (it is used for the `flows' table).
	Interval time.Duration
	// TTL is how long to keep data for this resolution. A
	// value of 0 means to never expire.
	TTL time.Duration
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
		Resolutions: []ResolutionConfiguration{
			{0, 6 * time.Hour},
			{time.Minute, 7 * 24 * time.Hour},
			{5 * time.Minute, 3 * 30 * 24 * time.Hour},
			{time.Hour, 6 * 30 * 24 * time.Hour},
		},
	}
}
