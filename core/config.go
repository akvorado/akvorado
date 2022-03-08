package core

// Configuration describes the configuration for the core component.
type Configuration struct {
	// Number of workers for the core component
	Workers int
}

// DefaultConfiguration represents the default configuration for the core component.
var DefaultConfiguration = Configuration{
	Workers: 1,
}
