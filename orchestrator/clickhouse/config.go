// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"reflect"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/helpers"
	"akvorado/common/kafka"

	"github.com/mitchellh/mapstructure"
)

// Configuration describes the configuration for the ClickHouse configurator.
type Configuration struct {
	clickhousedb.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Kafka describes Kafka-specific configuration
	Kafka KafkaConfiguration
	// Resolutions describe the various resolutions to use to
	// store data and the associated TTLs.
	Resolutions []ResolutionConfiguration
	// MaxPartitions define the number of partitions to have for a
	// consolidated flow tables when full.
	MaxPartitions int `validate:"isdefault|min=1"`
	// ASNs is a mapping from AS numbers to names. It replaces or
	// extends the builtin list of AS numbers.
	ASNs map[uint32]string
	// Networks is a mapping from IP networks to attributes. It is used
	// to instantiate the SrcNet* and DstNet* columns.
	Networks *helpers.SubnetMap[NetworkAttributes]
	// OrchestratorURL allows one to override URL to reach
	// orchestrator from Clickhouse
	OrchestratorURL string `validate:"isdefault|url"`
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
	Consumers int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the ClickHouse configurator.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration: clickhousedb.DefaultConfiguration(),
		Kafka: KafkaConfiguration{
			Consumers: 1,
		},
		Resolutions: []ResolutionConfiguration{
			{0, 15 * 24 * time.Hour},                   // 15 days
			{time.Minute, 7 * 24 * time.Hour},          // 7 days
			{5 * time.Minute, 3 * 30 * 24 * time.Hour}, // 90 days
			{time.Hour, 12 * 30 * 24 * time.Hour},      // 1 year
		},
		MaxPartitions: 50,
	}
}

// NetworkAttributes is a set of attributes attached to a network
type NetworkAttributes struct {
	// Name is a name attached to the network. May be unique or not.
	Name string
	// Role is a role attached to the network (server, customer).
	Role string
	// Site is the site of the network (paris, berlin).
	Site string
	// Region is the region of the network (france, italy).
	Region string
	// Tenant is a tenant for the network.
	Tenant string
}

// NetworkAttributesUnmarshallerHook decodes network attributes. It
// also accepts a string instead of attributes for backward
// compatibility.
func NetworkAttributesUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		from = helpers.ElemOrIdentity(from)
		to = helpers.ElemOrIdentity(to)
		if to.Type() != reflect.TypeOf(NetworkAttributes{}) {
			return from.Interface(), nil
		}
		if from.Kind() == reflect.String {
			return NetworkAttributes{Name: from.String()}, nil
		}
		return from.Interface(), nil
	}
}

func init() {
	helpers.AddMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[NetworkAttributes]())
	helpers.AddMapstructureUnmarshallerHook(NetworkAttributesUnmarshallerHook())
}
