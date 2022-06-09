package database

// Configuration describes the configuration for the authentication component.
type Configuration struct {
	// Driver defines the driver for the database
	Driver string
	// DSN defines the DSN to connect to the database
	DSN string
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	}
}
