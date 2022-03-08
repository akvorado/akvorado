package reporter

import (
	"akvorado/reporter/logger"
	"akvorado/reporter/metrics"
)

// Configuration contains the reporter configuration.
type Configuration struct {
	Logging logger.Configuration
	Metrics metrics.Configuration
}

// DefaultConfiguration is the default reporter configuration.
var DefaultConfiguration = Configuration{
	Logging: logger.DefaultConfiguration,
	Metrics: metrics.DefaultConfiguration,
}
