// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

// Configuration describes the configuration for the core component.
type Configuration struct {
	// Number of workers for the core component
	Workers int `validate:"min=1"`
	// ExporterClassifiers defines rules for exporter classification
	ExporterClassifiers []ExporterClassifierRule
	// InterfaceClassifiers defines rules for interface classification
	InterfaceClassifiers []InterfaceClassifierRule
	// ClassifierCacheSize defines the size of the classifier (in number of items)
	ClassifierCacheSize uint
	// Ignore source/dest AS numbers from received flows
	IgnoreASNFromFlow bool
}

// DefaultConfiguration represents the default configuration for the core component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Workers:              1,
		ExporterClassifiers:  []ExporterClassifierRule{},
		InterfaceClassifiers: []InterfaceClassifierRule{},
		ClassifierCacheSize:  1000,
		IgnoreASNFromFlow:    false,
	}
}
