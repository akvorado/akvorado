// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

// Configuration describes the configuration for the GeoIP component.
type Configuration struct {
	// ASNDatabase defines the path to the ASN database.
	ASNDatabase string
	// CountryDatabase defines the path to the country database.
	CountryDatabase string
	// Optional tells if we need to error if not present on start.
	Optional bool
}

// DefaultConfiguration represents the default configuration for the
// GeoIP component. Without databases, the component won't report
// anything.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
