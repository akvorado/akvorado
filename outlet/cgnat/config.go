// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cgnat

import "time"

// Configuration describes CGNAT cache behavior.
type Configuration struct {
	// Retention defines how long finished sessions are kept.
	Retention time.Duration `validate:"min=1m"`
	// CleanupInterval defines how often stale entries are removed.
	CleanupInterval time.Duration `validate:"min=1s"`
}

// DefaultConfiguration is the default configuration for the CGNAT component.
func DefaultConfiguration() Configuration {
	return Configuration{
		Retention:       24 * time.Hour,
		CleanupInterval: time.Minute,
	}
}
