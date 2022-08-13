// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"reflect"
	"time"

	"akvorado/common/helpers"

	"github.com/mitchellh/mapstructure"
)

// Configuration describes the configuration for the SNMP client
type Configuration struct {
	// CacheDuration defines how long to keep cached entries without access
	CacheDuration time.Duration `validate:"min=1m"`
	// CacheRefresh defines how soon to refresh an existing cached entry
	CacheRefresh time.Duration `validate:"eq=0|min=1m,eq=0|gtefield=CacheDuration"`
	// CacheRefreshInterval defines the interval to check for expiration/refresh
	CacheCheckInterval time.Duration `validate:"ltefield=CacheRefresh"`
	// CachePersist defines a file to store cache and survive restarts
	CachePersistFile string
	// PollerRetries tell how many time a poller should retry before giving up
	PollerRetries int `validate:"min=0"`
	// PollerTimeout tell how much time a poller should wait for an answer
	PollerTimeout time.Duration
	// PollerCoalesce tells how many requests can be contained inside a single SNMP PDU
	PollerCoalesce int `validate:"min=0"`
	// Workers define the number of workers used to poll SNMP
	Workers int `validate:"min=1"`

	// Communities is a mapping from exporter IPs to communities
	Communities *helpers.SubnetMap[string]
}

// DefaultConfiguration represents the default configuration for the SNMP client.
func DefaultConfiguration() Configuration {
	communities, err := helpers.NewSubnetMap(map[string]string{
		"::/0": "public",
	})
	if err != nil {
		panic(err)
	}
	return Configuration{
		CacheDuration:      30 * time.Minute,
		CacheRefresh:       time.Hour,
		CacheCheckInterval: 2 * time.Minute,
		CachePersistFile:   "",
		Communities:        communities,
		PollerRetries:      1,
		PollerTimeout:      time.Second,
		PollerCoalesce:     10,
		Workers:            1,
	}
}

// ConfigurationUnmarshallerHook normalize SNMP configuration:
//   - append default-community to communities (as ::/0)
//   - ensure we have a default value for communities
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || from.Type().Key().Kind() != reflect.String || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// default-community → communities
		var defaultKey, mapKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			if helpers.MapStructureMatchName(k.String(), "DefaultCommunity") {
				defaultKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "Communities") {
				mapKey = &fromMap[i]
			}
		}
		var communities reflect.Value
		if mapKey != nil {
			communities = helpers.ElemOrIdentity(from.MapIndex(*mapKey))
		}
		if defaultKey != nil && !helpers.ElemOrIdentity(from.MapIndex(*defaultKey)).IsZero() {
			if mapKey == nil {
				// Use the fact we can set the default value directly.
				from.SetMapIndex(reflect.ValueOf("communities"), from.MapIndex(*defaultKey))
			} else {
				communities.SetMapIndex(reflect.ValueOf("::/0"), from.MapIndex(*defaultKey))
			}
		} else {
			// default-community should contain ::/0
			if mapKey == nil {
				from.SetMapIndex(reflect.ValueOf("communities"), reflect.ValueOf("public"))
			} else if !communities.MapIndex(reflect.ValueOf("::/0")).IsValid() {
				communities.SetMapIndex(reflect.ValueOf("::/0"), reflect.ValueOf("public"))
			}
		}
		if defaultKey != nil {
			from.SetMapIndex(*defaultKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.AddMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.AddMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[string]())
}
