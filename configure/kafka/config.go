package kafka

import "akvorado/common/kafka"

// Configuration describes the configuration for the Kafka configurator.
type Configuration struct {
	// Connect describes how to connect to Kafka.
	Connect kafka.Configuration
	// TopicConfiguration describes the topic configuration.
	TopicConfiguration TopicConfiguration
}

// TopicConfiguration describes the configuration for a topic
type TopicConfiguration struct {
	// NumPartitions tells how many partitions should be used for the topic.
	NumPartitions int32
	// ReplicationFactor tells the replication factor for the topic.
	ReplicationFactor int16
	// ConfigEntries is a map to specify the topic overrides. Non-listed overrides will be removed
	ConfigEntries map[string]*string
}

// DefaultConfiguration represents the default configuration for the Kafka configurator.
var DefaultConfiguration = Configuration{
	Connect: kafka.DefaultConfiguration,
	TopicConfiguration: TopicConfiguration{
		NumPartitions:     1,
		ReplicationFactor: 1,
	},
}
