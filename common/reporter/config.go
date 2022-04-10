package reporter

import (
	"akvorado/common/reporter/logger"
	"akvorado/common/reporter/metrics"
)

// Configuration contains the reporter configuration.
type Configuration struct {
	Logging logger.Configuration
	Metrics metrics.Configuration
}

// DefaultConfiguration is the default reporter configuration.
func DefaultConfiguration() Configuration {
	return Configuration{
		Logging: logger.DefaultConfiguration(),
		Metrics: metrics.DefaultConfiguration(),
	}
}
