// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"akvorado/common/helpers"
)

// Configuration describes the configuration for the GeoIP component.
type Configuration struct {
	// ASNDatabase defines the path to the ASN database.
	ASNDatabase []string
	// GeoDatabase defines the path to the geo database.
	GeoDatabase []string
	// Optional tells if we need to error if not present on start.
	Optional bool
}

// DefaultConfiguration represents the default configuration for the
// GeoIP component. Without databases, the component won't report
// anything.
func DefaultConfiguration() Configuration {
	return Configuration{}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.RenameKeyUnmarshallerHook(Configuration{}, "CountryDatabase", "GeoDatabase"))
}
