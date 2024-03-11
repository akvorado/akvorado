// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"reflect"
	"time"

	"akvorado/common/remotedatasourcefetcher"

	"akvorado/common/clickhousedb"
	"akvorado/common/helpers"
	"akvorado/common/kafka"
	"akvorado/orchestrator/clickhouse/geoip"

	"github.com/mitchellh/mapstructure"
)

// Configuration describes the configuration for the ClickHouse configurator.
type Configuration struct {
	clickhousedb.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// SkipMigrations tell if we should skip migrations.
	SkipMigrations bool
	// Kafka describes Kafka-specific configuration
	Kafka KafkaConfiguration
	// Resolutions describe the various resolutions to use to
	// store data and the associated TTLs.
	Resolutions []ResolutionConfiguration `validate:"min=1,dive"`
	// MaxPartitions define the number of partitions to have for a
	// consolidated flow tables when full.
	MaxPartitions int `validate:"isdefault|min=1"`
	// SystemLogTTL is the TTL to set for system log tables.
	SystemLogTTL time.Duration `validate:"isdefault|min=1m"`
	// PrometheusEndpoint defines the endpoint ClickHouse can use to expose
	// metrics to Prometheus. If not defined, this is not configured.
	PrometheusEndpoint string
	// ASNs is a mapping from AS numbers to names. It replaces or
	// extends the builtin list of AS numbers.
	ASNs map[uint32]string
	// Networks is a mapping from IP networks to attributes. It is used
	// to instantiate the SrcNet* and DstNet* columns.
	Networks *helpers.SubnetMap[NetworkAttributes] `validate:"omitempty,dive"`
	// NetworkSources defines a set of remote network
	// definitions to map IP networks to attributes. It is used to
	// instantiate the SrcNet* and DstNet* columns. The results
	// are overridden by the content of Networks.
	NetworkSources map[string]remotedatasourcefetcher.RemoteDataSource `validate:"dive"`
	// NetworkSourceTimeout tells how long to wait for network
	// sources to be ready. 503 is returned when not.
	NetworkSourcesTimeout time.Duration `validate:"min=0"`
	// OrchestratorURL allows one to override URL to reach
	// orchestrator from ClickHouse
	OrchestratorURL string `validate:"isdefault|url"`

	GeoIP geoip.Configuration
}

// ResolutionConfiguration describes a consolidation interval.
type ResolutionConfiguration struct {
	// Interval is the consolidation interval for this
	// resolution. An interval of 0 means no consolidation
	// takes place (it is used for the `flows' table).
	Interval time.Duration `validate:"isdefault|min=5s"`
	// TTL is how long to keep data for this resolution. A
	// value of 0 means to never expire.
	TTL time.Duration `validate:"isdefault|min=1h"`
}

// KafkaConfiguration describes Kafka-specific configuration
type KafkaConfiguration struct {
	kafka.Configuration `mapstructure:",squash" yaml:"-,inline"`
	// Consumers tell how many consumers to use to poll data from Kafka
	Consumers int `validate:"min=1"`
	// GroupName defines the Kafka consumers group used to poll data from topic,
	// shared between all Consumers.
	GroupName string
	// EngineSettings allows one to set arbitrary settings for Kafka engine in
	// ClickHouse.
	EngineSettings []string
}

// DefaultConfiguration represents the default configuration for the ClickHouse configurator.
func DefaultConfiguration() Configuration {
	return Configuration{
		Configuration: clickhousedb.DefaultConfiguration(),
		Kafka: KafkaConfiguration{
			Consumers: 1,
			GroupName: "clickhouse",
		},
		Resolutions: []ResolutionConfiguration{
			{0, 15 * 24 * time.Hour},                   // 15 days
			{time.Minute, 7 * 24 * time.Hour},          // 7 days
			{5 * time.Minute, 3 * 30 * 24 * time.Hour}, // 90 days
			{time.Hour, 12 * 30 * 24 * time.Hour},      // 1 year
		},
		MaxPartitions:         50,
		NetworkSourcesTimeout: 10 * time.Second,
		SystemLogTTL:          30 * 24 * time.Hour, // 30 days
	}
}

// NetworkAttributes is a set of attributes attached to a network.
// Don't forget to update orchestrator/clickhouse/migrations.go:78 when this changes.
type NetworkAttributes struct {
	// Name is a name attached to the network. May be unique or not.
	Name string
	// Role is a role attached to the network (server, customer).
	Role string
	// Site is the site of the network (ams5, pa3).
	Site string
	// Region is the region of the network (eu-west-1, us-east-3).
	Region string
	// City is the administrative city where the prefix is located (Paris, London).
	City string
	// State is the first administrative sub-division of the country (Ile-de-france, Alabama)
	State string
	// Country is the country of the network (france, italy)
	Country string
	// Tenant is a tenant for the network.
	Tenant string
	// ASN is the AS number associated to the network.
	ASN uint32
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
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[NetworkAttributes]())
	helpers.RegisterMapstructureUnmarshallerHook(NetworkAttributesUnmarshallerHook())
	helpers.RegisterSubnetMapValidation[NetworkAttributes]()
}
