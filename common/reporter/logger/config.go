package logger

// Configuration if the configuration for logger. Currently, there is no configuration.
type Configuration struct{}

// DefaultConfiguration is the default logging configuration.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
