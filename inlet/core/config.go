// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/helpers/bimap"

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
	// RISProvider defines the BMP source to use
	RISProvider RISProvider
	// Old configuration settings
	classifierCacheSize uint
}

// DefaultConfiguration represents the default configuration for the core component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Workers:                 1,
		ExporterClassifiers:     []ExporterClassifierRule{},
		InterfaceClassifiers:    []InterfaceClassifierRule{},
		ClassifierCacheDuration: 5 * time.Minute,
		ASNProviders:            []ASNProvider{ASNProviderFlow, ASNProviderBMP, ASNProviderGeoIP},
		NetProviders:            []NetProvider{NetProviderFlow, NetProviderBMP},
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
	// ASNProviderBMP uses the AS number from BMP
	ASNProviderBMP
	// ASNProviderBMPExceptPrivate uses the AS number from BMP, except if this is a private AS.
	ASNProviderBMPExceptPrivate
)

var asnProviderMap = bimap.New(map[ASNProvider]string{
	ASNProviderFlow:              "flow",
	ASNProviderFlowExceptPrivate: "flow-except-private",
	ASNProviderGeoIP:             "geoip",
	ASNProviderBMP:               "bmp",
	ASNProviderBMPExceptPrivate:  "bmp-except-private",
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

const (
	// NetProviderFlow uses the network mask embedded in flows, if any
	NetProviderFlow NetProvider = iota
	// NetProviderBMP uses looks the netmask up with BMP
	NetProviderBMP
)

var netProviderMap = bimap.New(map[NetProvider]string{
	NetProviderFlow: "flow",
	NetProviderBMP:  "bmp",
})

// MarshalText turns an AS provider to text.
func (np NetProvider) MarshalText() ([]byte, error) {
	got, ok := netProviderMap.LoadValue(np)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown field")
}

// String turns an AS provider to string.
func (np NetProvider) String() string {
	got, _ := netProviderMap.LoadValue(np)
	return got
}

// UnmarshalText provides an AS provider from a string.
func (np *NetProvider) UnmarshalText(input []byte) error {
	got, ok := netProviderMap.LoadKey(string(input))
	if ok {
		*np = got
		return nil
	}
	return errors.New("unknown provider")
}

// RISProvider describes one BMP/RIS number provider.
type RISProvider int

const (
	// RISProviderInternal uses akvorado's own provider for ris
	RISProviderInternal RISProvider = iota
	// RISProviderBioRis pulls the data from a Bio Routing RIS Client
	RISProviderBioRis
)

var risProviderMap = bimap.New(map[RISProvider]string{
	RISProviderInternal: "internal",
	RISProviderBioRis:   "bioris",
})

// MarshalText turns an RIS provider to text.
func (rp RISProvider) MarshalText() ([]byte, error) {
	got, ok := risProviderMap.LoadValue(rp)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown field")
}

// String turns an RIS provider to string.
func (rp RISProvider) String() string {
	got, _ := risProviderMap.LoadValue(rp)
	return got
}

// UnmarshalText provides an AS provider from a string.
func (rp *RISProvider) UnmarshalText(input []byte) error {
	got, ok := risProviderMap.LoadKey(string(input))
	if ok {
		*rp = got
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
					reflect.ValueOf([]ASNProvider{ASNProviderGeoIP}))
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
