package s3

import (
	"time"

	"akvorado/common/helpers"
)

// Configuration describes the configuration for the S3 component.
type Configuration struct {
	S3Config map[string]ConfigEntry `validate:"omitempty,dive"`
}

// ConfigEntry describes the configuration for a single S3 bucket usage.
type ConfigEntry struct {
	Credentials Credentials `validate:"required"`
	Bucket      string      `validate:"required"`
	Prefix      string
	Timeout     time.Duration
	EndpointURL string
	Mock        bool
	PathStyle   bool
}

// Credentials holds the credentials for an S3 bucket. More credential providers might be added in the future.
type Credentials struct {
	Region string
}

// DefaultConfiguration is the default configuration of the s3 client
func DefaultConfiguration() Configuration {
	return Configuration{
		S3Config: map[string]ConfigEntry{},
	}
}

// DefaultConfigEntryConfiguration is the default configuration of a single s3 client/bucket
func DefaultConfigEntryConfiguration() ConfigEntry {
	return ConfigEntry{
		Timeout: time.Second,
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(helpers.DefaultValuesUnmarshallerHook[ConfigEntry](DefaultConfigEntryConfiguration()))
}
