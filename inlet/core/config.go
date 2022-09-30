// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"errors"
	"fmt"
	"reflect"

	"akvorado/common/helpers"

	"github.com/mitchellh/mapstructure"
)

// Configuration describes the configuration for the core component.
type Configuration struct {
	// Number of workers for the core component
	Workers int `validate:"min=1"`
	// ExporterClassifiers defines rules for exporter classification
	ExporterClassifiers []ExporterClassifierRule
	// InterfaceClassifiers defines rules for interface classification
	InterfaceClassifiers []InterfaceClassifierRule
	// ClassifierCacheSize defines the size of the classifier (in number of items)
	ClassifierCacheSize uint
	// DefaultSamplingRate defines the default sampling rate to use when the information is missing
	DefaultSamplingRate helpers.SubnetMap[uint]
	// OverrideSamplingRate defines a sampling rate to use instead of the received on
	OverrideSamplingRate helpers.SubnetMap[uint]
	// ASNProviders defines the source used to get AS numbers
	ASNProviders []ASNProvider `validate:"dive"`
}

// DefaultConfiguration represents the default configuration for the core component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Workers:              1,
		ExporterClassifiers:  []ExporterClassifierRule{},
		InterfaceClassifiers: []InterfaceClassifierRule{},
		ClassifierCacheSize:  1000,
		ASNProviders:         []ASNProvider{ProviderFlow, ProviderBMP, ProviderGeoIP},
	}
}

// ASNProvider describes one AS number provider.
type ASNProvider int

const (
	// ProviderFlow uses the AS number embedded in flows.
	ProviderFlow ASNProvider = iota
	// ProviderFlowExceptPrivate uses the AS number embedded in flows, except if this is a private AS.
	ProviderFlowExceptPrivate
	// ProviderGeoIP pulls the AS number from a GeoIP database.
	ProviderGeoIP
	// ProviderBMP uses the AS number from BMP
	ProviderBMP
	// ProviderBMPExceptPrivate uses the AS number from BMP, except if this is a private AS.
	ProviderBMPExceptPrivate
)

var asnProviderMap = helpers.NewBimap(map[ASNProvider]string{
	ProviderFlow:              "flow",
	ProviderFlowExceptPrivate: "flow-except-private",
	ProviderGeoIP:             "geoip",
	ProviderBMP:               "bmp",
	ProviderBMPExceptPrivate:  "bmp-except-private",
})

// MarshalText turns an AS provider to text.
func (ap ASNProvider) MarshalText() ([]byte, error) {
	got, ok := asnProviderMap.LoadValue(ap)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown field")
}

// String turns an AS provider to string.
func (ap ASNProvider) String() string {
	got, _ := asnProviderMap.LoadValue(ap)
	return got
}

// UnmarshalText provides an AS provider from a string.
func (ap *ASNProvider) UnmarshalText(input []byte) error {
	got, ok := asnProviderMap.LoadKey(string(input))
	if ok {
		*ap = got
		return nil
	}
	return errors.New("unknown provider")
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
				from.SetMapIndex(reflect.ValueOf("asn-providers"),
					reflect.ValueOf([]ASNProvider{ProviderGeoIP}))
			}
			from.SetMapIndex(*oldKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[uint]())
}
