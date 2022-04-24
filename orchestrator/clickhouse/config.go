package clickhouse

import (
	"fmt"
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
	// takes place (it is used for the `flow' table.
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
			{10 * time.Second, 24 * time.Hour},
			{time.Minute, 7 * 24 * time.Hour},
			{5 * time.Minute, 3 * 30 * 24 * time.Hour},
			{time.Hour, 6 * 30 * 24 * time.Hour},
		},
	}
}

// resolutionsToTTL converts a set of resolutions to a TTL
// clause. It is assumed the first resolution is of an interval of 0.
func resolutionsToTTL(resolutions []ResolutionConfiguration, groupBy string) []string {
	// Build TTL clause
	ttl := []string{}
	for idx := 1; idx < len(resolutions); idx++ {
		toStart := fmt.Sprintf("toStartOfInterval(TimeReceived, INTERVAL %d second)",
			uint64(resolutions[idx].Interval.Seconds()))
		set := fmt.Sprintf("Bytes = SUM(Bytes), Packets = SUM(Packets), TimeReceived = %s", toStart)
		ttl = append(ttl, fmt.Sprintf("TimeReceived + INTERVAL %d second GROUP BY %s SET %s",
			uint64(resolutions[idx-1].TTL.Seconds()), groupBy, set))
	}
	ttl = append(ttl, fmt.Sprintf("TimeReceived + INTERVAL %d second DELETE",
		uint64(resolutions[len(resolutions)-1].TTL.Seconds())))
	return ttl
}
