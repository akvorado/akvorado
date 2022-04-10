package file

import "akvorado/inlet/flow/input"

// Configuration describes file input configuration.
type Configuration struct {
	// Paths to use as input
	Paths []string
}

// DefaultConfiguration descrives the default configuration for file input.
func DefaultConfiguration() input.Configuration {
	return &Configuration{}
}
