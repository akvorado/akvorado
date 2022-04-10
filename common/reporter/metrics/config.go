package metrics

// Configuration is currently empty as this sub-component is not
// configurable yet.
type Configuration struct{}

// DefaultConfiguration is the default metrics configuration.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
