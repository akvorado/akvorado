// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"time"

	"akvorado/common/clickhousedb"
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
	// ASNs is a mapping from AS numbers to names. It replaces or
	// extends the builtin list of AS numbers.
	ASNs map[uint32]string
	// Networks is a mapping from IP networks to names. It is used
	// to instantiate the SrcNetName and DstNetName columns.
	Networks NetworkNames
	// OrchestratorURL allows one to override URL to reach
	// orchestrator from Clickhouse
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
			{0, 15 * 24 * time.Hour},
			{time.Minute, 7 * 24 * time.Hour},
			{5 * time.Minute, 3 * 30 * 24 * time.Hour},
			{time.Hour, 12 * 30 * 24 * time.Hour},
		},
	}
}

// NetworkNames is a mapping from a network to a name.
type NetworkNames map[string]string

// NetworkNamesUnmarshalerHook decodes NetworkNames mapping and notably check that valid networks are provided as key.
func NetworkNamesUnmarshalerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data interface{}) (interface{}, error) {
		if from != reflect.TypeOf(map[string]string{}) || to != reflect.TypeOf(NetworkNames{}) {
			return data, nil
		}
		input := data.(map[string]string)
		output := NetworkNames{}
		if input == nil {
			input = map[string]string{}
		}
		for k, v := range input {
			// Parse
			_, ipNet, err := net.ParseCIDR(k)
			if err != nil {
				return nil, err
			}
			// Convert to IPv6
			ones, bits := ipNet.Mask.Size()
			if bits != 32 && bits != 128 {
				return nil, errors.New("invalid netmask")
			}
			if bits == 32 {
				output[fmt.Sprintf("::ffff:%s/%d", ipNet.IP.String(), ones+96)] = v
			} else {
				output[ipNet.String()] = v
			}
		}
		return output, nil
	}
}
