package core

// Configuration describes the configuration for the core component.
type Configuration struct {
	// Number of workers for the core component
	Workers int
	// SamplerClassifiers defines rules for sampler classification
	SamplerClassifiers []SamplerClassifierRule
	// InterfaceClassifiers defines rules for interface classification
	InterfaceClassifiers []InterfaceClassifierRule
	// ClassifierCacheSize defines the size of the classifier (in number of items)
	ClassifierCacheSize uint
}

// DefaultConfiguration represents the default configuration for the core component.
var DefaultConfiguration = Configuration{
	Workers:              1,
	SamplerClassifiers:   []SamplerClassifierRule{},
	InterfaceClassifiers: []InterfaceClassifierRule{},
	ClassifierCacheSize:  1000,
}
