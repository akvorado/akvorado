package console

import "akvorado/console/authentication"

// Configuration describes the configuration for the console component.
type Configuration struct {
	// ServeLiveFS serve files from the filesystem instead of the embedded versions.
	ServeLiveFS bool
	// Authentication describes authentication configuration
	Authentication authentication.Configuration
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Authentication: authentication.DefaultConfiguration(),
	}
}
