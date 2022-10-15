// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package database

// Configuration describes the configuration for the authentication component.
type Configuration struct {
	// Driver defines the driver for the database
	Driver string `validate:"required"`
	// DSN defines the DSN to connect to the database
	DSN string `validate:"required"`
	// SavedFilters is a list of saved filters to include for all users
	SavedFilters []BuiltinSavedFilter `validate:"dive"`
}

// DefaultConfiguration represents the default configuration for the console component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
	}
}

// BuiltinSavedFilter is a saved filter
type BuiltinSavedFilter struct {
	Description string `validate:"required"`
	Content     string `validate:"required"`
}
