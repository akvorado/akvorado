// Package ddos implements a simple DDoS detection module.
package ddos

import "time"

// Configuration describes the settings for DDoS detection.
type Configuration struct {
	Enabled         bool          `yaml:"enabled"`
	DetectionWindow time.Duration `yaml:"detection-window" validate:"min=1s"`
	MinFlows        uint64        `yaml:"min-flows"`
}

// DefaultConfiguration returns default values for Configuration.
func DefaultConfiguration() Configuration {
	return Configuration{
		Enabled:         false,
		DetectionWindow: 10 * time.Second,
		MinFlows:        1000,
	}
}
