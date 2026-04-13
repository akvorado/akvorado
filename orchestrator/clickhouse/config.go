// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"fmt"
	"reflect"
	"time"

	"akvorado/common/remotedatasource"

	"akvorado/common/helpers"

	"github.com/go-viper/mapstructure/v2"
)

// Configuration describes the configuration for the ClickHouse configurator.
type Configuration struct {
	// SkipMigrations tell if we should skip migrations.
	SkipMigrations bool
	// Resolutions describe the various resolutions to use to
	// store data and the associated TTLs.
	Resolutions []ResolutionConfiguration `validate:"min=1,dive"`
	// MaxPartitions define the number of partitions to have for a
	// consolidated flow tables when full.
	MaxPartitions int `validate:"isdefault|min=1"`
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
	NetworkSources map[string]remotedatasource.Source `validate:"dive"`
	// NetworkSourceTimeout tells how long to wait for network
	// sources to be ready. 503 is returned when not.
	NetworkSourcesTimeout time.Duration `validate:"min=0"`
	// OrchestratorURL allows one to override URL to reach
	// orchestrator from ClickHouse
	OrchestratorURL string `validate:"isdefault|url"`
	// OrchestratorBasicAuth holds optional basic auth credentials to reach
	// orchestrator from ClickHouse
	OrchestratorBasicAuth *ConfigurationBasicAuth
}

// ConfigurationBasicAuth holds Username and Password subfields
// for basicauth purposes
type ConfigurationBasicAuth struct {
	Username string `validate:"min=1"`
	Password string `validate:"min=1"`
}

// TableSettings is a map of ClickHouse table settings.
// Values should be integers or strings.
type TableSettings map[string]any

// ResolutionConfiguration describes a consolidation interval.
type ResolutionConfiguration struct {
	// Interval is the consolidation interval for this
	// resolution. An interval of 0 means no consolidation
	// takes place (it is used for the `flows' table).
	Interval time.Duration `validate:"isdefault|min=5s"`
	// TTL is how long to keep data for this resolution. A
	// value of 0 means to never expire.
	TTL time.Duration `validate:"isdefault|min=1h"`
	// TableSettings is a map of additional ClickHouse table settings
	// to apply. These are merged with the default settings
	// (index_granularity=8192, ttl_only_drop_parts=1).
	TableSettings TableSettings `validate:"dive,keys,alphanumunderscore,endkeys"`
}

// DefaultConfiguration represents the default configuration for the ClickHouse configurator.
func DefaultConfiguration() Configuration {
	return Configuration{
		Resolutions: []ResolutionConfiguration{
			{Interval: 0, TTL: 15 * 24 * time.Hour},                   // 15 days
			{Interval: time.Minute, TTL: 7 * 24 * time.Hour},          // 7 days
			{Interval: 5 * time.Minute, TTL: 3 * 30 * 24 * time.Hour}, // 90 days
			{Interval: time.Hour, TTL: 12 * 30 * 24 * time.Hour},      // 1 year
		},
		MaxPartitions:         50,
		NetworkSourcesTimeout: 10 * time.Second,
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
	return func(from, to reflect.Value) (any, error) {
		from = helpers.ElemOrIdentity(from)
		to = helpers.ElemOrIdentity(to)
		if to.Type() != reflect.TypeFor[NetworkAttributes]() {
			return from.Interface(), nil
		}
		if from.Kind() == reflect.String {
			return NetworkAttributes{Name: from.String()}, nil
		}
		return from.Interface(), nil
	}
}

// TableSettingsUnmarshallerHook decodes TableSettings values into int or string.
func TableSettingsUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (any, error) {
		from = helpers.ElemOrIdentity(from)
		to = helpers.ElemOrIdentity(to)
		if to.Type() != reflect.TypeFor[TableSettings]() {
			return from.Interface(), nil
		}
		if from.Kind() != reflect.Map {
			return from.Interface(), nil
		}
		result := TableSettings{}
		for _, key := range from.MapKeys() {
			k := helpers.ElemOrIdentity(key)
			if k.Kind() != reflect.String {
				return nil, fmt.Errorf("table setting key must be a string, got %s", k.Kind())
			}
			v := helpers.ElemOrIdentity(from.MapIndex(key))
			// It's important to output the same types than in `renderTableSettings`.
			switch v.Kind() {
			case reflect.String:
				result[k.String()] = v.String()
			case reflect.Int:
				result[k.String()] = int(v.Int())
			default:
				return nil, fmt.Errorf("table setting %q must be a string or integer, got %s", k.String(), v.Kind())
			}
		}
		return result, nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[NetworkAttributes]())
	helpers.RegisterMapstructureUnmarshallerHook(NetworkAttributesUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(TableSettingsUnmarshallerHook())
	helpers.RegisterMapstructureDeprecatedFields[Configuration](
		"SystemLogTTL",
		"PrometheusEndpoint",
		"Kafka")
	helpers.RegisterSubnetMapValidation[NetworkAttributes]()
}
