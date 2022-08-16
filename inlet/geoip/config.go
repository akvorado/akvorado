// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"fmt"
	"reflect"

	"akvorado/common/helpers"

	"github.com/mitchellh/mapstructure"
)

// Configuration describes the configuration for the GeoIP component.
type Configuration struct {
	// ASNDatabase defines the path to the ASN database.
	ASNDatabase string
	// GeoDatabase defines the path to the geo database.
	GeoDatabase string
	// Optional tells if we need to error if not present on start.
	Optional bool
}

// DefaultConfiguration represents the default configuration for the
// GeoIP component. Without databases, the component won't report
// anything.
func DefaultConfiguration() Configuration {
	return Configuration{}
}

// ConfigurationUnmarshallerHook normalize GeoIP configuration:
//   - replace country-database by geo-database
func ConfigurationUnmarshallerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Value) (interface{}, error) {
		if from.Kind() != reflect.Map || from.IsNil() || to.Type() != reflect.TypeOf(Configuration{}) {
			return from.Interface(), nil
		}

		// country-database → geo-database
		var countryKey, geoKey *reflect.Value
		fromMap := from.MapKeys()
		for i, k := range fromMap {
			k = helpers.ElemOrIdentity(k)
			if k.Kind() != reflect.String {
				return from.Interface(), nil
			}
			if helpers.MapStructureMatchName(k.String(), "CountryDatabase") {
				countryKey = &fromMap[i]
			} else if helpers.MapStructureMatchName(k.String(), "GeoDatabase") {
				geoKey = &fromMap[i]
			}
		}
		if countryKey != nil && geoKey != nil {
			return nil, fmt.Errorf("cannot have both %q and %q", countryKey.String(), geoKey.String())
		}
		if countryKey != nil {
			from.SetMapIndex(reflect.ValueOf("geo-database"), from.MapIndex(*countryKey))
			from.SetMapIndex(*countryKey, reflect.Value{})
		}

		return from.Interface(), nil
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(ConfigurationUnmarshallerHook())
}
