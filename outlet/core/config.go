// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"akvorado/common/helpers"

	"github.com/go-viper/mapstructure/v2"
)

// Configuration describes the configuration for the core component.
type Configuration struct {
	// ExporterClassifiers defines rules for exporter classification
	ExporterClassifiers []ExporterClassifierRule
	// InterfaceClassifiers defines rules for interface classification
	InterfaceClassifiers []InterfaceClassifierRule
	// ClassifierCacheDuration defines the default TTL for classifier cache
	ClassifierCacheDuration time.Duration `validate:"min=1s"`
	// DefaultSamplingRate defines the default sampling rate to use when the information is missing
	DefaultSamplingRate helpers.SubnetMap[uint]
	// OverrideSamplingRate defines a sampling rate to use instead of the received on
	OverrideSamplingRate helpers.SubnetMap[uint]
	// ASNProviders defines the source used to get AS numbers
	ASNProviders []ASNProvider `validate:"dive"`
	// NetProviders defines the source used to get Prefix/Network Information
	NetProviders []NetProvider `validate:"dive"`
	// Old configuration settings
	classifierCacheSize uint
}

// DefaultConfiguration represents the default configuration for the core component.
func DefaultConfiguration() Configuration {
	return Configuration{
		ExporterClassifiers:     []ExporterClassifierRule{},
		InterfaceClassifiers:    []InterfaceClassifierRule{},
		ClassifierCacheDuration: 5 * time.Minute,
		ASNProviders:            []ASNProvider{ASNProviderFlow, ASNProviderRouting, ASNProviderGeoIP},
		NetProviders:            []NetProvider{NetProviderFlow, NetProviderRouting},
	}
}

type (
	// ASNProvider describes one AS number provider.
	ASNProvider int
	// NetProvider describes one network mask provider.
	NetProvider int
)

const (
	// ASNProviderFlow uses the AS number embedded in flows.
	ASNProviderFlow ASNProvider = iota
	// ASNProviderFlowExceptPrivate uses the AS number embedded in flows, except if this is a private AS.
	ASNProviderFlowExceptPrivate
	// ASNProviderGeoIP pulls the AS number from a GeoIP database.
	ASNProviderGeoIP
	// ASNProviderRouting uses the AS number from BMP
	ASNProviderRouting
	// ASNProviderRoutingExceptPrivate uses the AS number from BMP, except if this is a private AS.
	ASNProviderRoutingExceptPrivate
)

const (
	// NetProviderFlow uses the network mask embedded in flows, if any
	NetProviderFlow NetProvider = iota
	// NetProviderRouting uses looks the netmask up with BMP
	NetProviderRouting
)

// ASNProviderUnmarshallerHook normalize a net provider configuration:
//   - map bmp to routing
func ASNProviderUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.String || to.Type() != reflect.TypeOf(ASNProvider(0)) {
			return from.Interface(), nil
		}
		if strings.ToLower(from.String()) == "bmp" {
			return "routing", nil
		}
		if strings.ToLower(from.String()) == "bmp-except-private" {
			return "routing-except-private", nil
		}
		return from.Interface(), nil
	}
}

// NetProviderUnmarshallerHook normalize a net provider configuration:
//   - map bmp to routing
func NetProviderUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.String || to.Type() != reflect.TypeOf(NetProvider(0)) {
			return from.Interface(), nil
		}
		if strings.ToLower(from.String()) == "bmp" {
			return "routing", nil
		}
		return from.Interface(), nil
	}
}

// ConfigurationUnmarshallerHook normalize core configuration:
//   - replace ignore-asn-from-flow by asn-providers
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// ignore-asn-from-flow → asn-providers
		var oldKey, newKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if helpers.MapStructureMatchName(k.String(), "IgnoreASNFromFlow") {
				oldKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "ASNProviders") {
				newKey = &fromMap[i]
			}
		}
		if oldKey != nil && newKey != nil {
			return nil, fmt.Errorf("cannot have both %q and %q", oldKey.String(), newKey.String())
		}
		if oldKey != nil {
			oldValue := helpers.ElemOrIdentity(from.MapIndex(*oldKey))
			if oldValue.Kind() == reflect.Bool && oldValue.Bool() == true {
				newASNProviders := []ASNProvider{}
				for _, p := range DefaultConfiguration().ASNProviders {
					if p != ASNProviderFlow && p != ASNProviderFlowExceptPrivate {
						newASNProviders = append(newASNProviders, p)
					}
				}
				from.SetMapIndex(reflect.ValueOf("asn-providers"), reflect.ValueOf(newASNProviders))
			}
			from.SetMapIndex(*oldKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(ASNProviderUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(NetProviderUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[uint]())
	helpers.RegisterMapstructureDeprecatedFields[Configuration]("Workers")
}
