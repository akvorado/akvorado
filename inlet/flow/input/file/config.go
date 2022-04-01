package file

// Configuration describes file input configuration.
type Configuration struct {
	// Paths to use as input
	Paths []string
}

// DefaultConfiguration descrives the default configuration for file input.
var DefaultConfiguration Configuration
