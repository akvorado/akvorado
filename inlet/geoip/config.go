package geoip

// Configuration describes the configuration for the GeoIP component.
type Configuration struct {
	// ASNDatabase defines the path to the ASN database.
	ASNDatabase string
	// CountryDatabase defines the path to the country database.
	CountryDatabase string
}

// DefaultConfiguration represents the default configuration for the
// GeoIP component. Without databases, the component won't report
// anything.
var DefaultConfiguration = Configuration{}
