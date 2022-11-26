// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"errors"
	"strings"
	"time"
)

// Configuration describes the configuration for the BMP server.
type Configuration struct {
	// Listen tells on which port the BMP server should listen to.
	Listen string `validate:"listen"`
	// RDs list the RDs to keep. If none are specified, all
	// received routes are processed. 0 match an absence of RD.
	RDs []RD
	// CollectASNs is true when we want to collect origin AS numbers
	CollectASNs bool
	// CollectASPaths is true when we want to collect AS paths
	CollectASPaths bool
	// CollectCommunities is true when we want to collect communities
	CollectCommunities bool
	// Keep tells how long to keep routes from a BMP client when it goes down
	Keep time.Duration `validate:"min=1s"`
	// RIBMode tells which mode to use for the RIB
	RIBMode RIBMode
	// RIBIdleUpdateDelay tells to update the read-only RIB after being idle for
	// that duration. This is only when RIB is in performance mode.
	RIBIdleUpdateDelay time.Duration `validate:"min=1s"`
	// RIBMinimumUpdateDelay tells to not update the read-only RIB less than
	// that. This is only when RIB is in performance mode.
	RIBMinimumUpdateDelay time.Duration `validate:"min=1s,gtfield=RIBIdleUpdateDelay"`
	// RIBMaximumUpdateDelay tells to update the read-only RIB at least once
	// every the specified delay (if there are updates). This is only if RIB is
	// in performance mode.
	RIBMaximumUpdateDelay time.Duration `validate:"min=1s,gtfield=RIBMinimumUpdateDelay"`
	// RIBPeerRemovalBatchRoutes tells how many routes to remove before checking
	// if we have a higher priority request. This is only if RIB is in memory
	// mode.
	RIBPeerRemovalBatchRoutes int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the BMP server
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen:                    "0.0.0.0:10179",
		CollectASNs:               true,
		CollectASPaths:            true,
		CollectCommunities:        true,
		Keep:                      5 * time.Minute,
		RIBMode:                   RIBModeMemory,
		RIBIdleUpdateDelay:        5 * time.Second,
		RIBMinimumUpdateDelay:     20 * time.Second,
		RIBMaximumUpdateDelay:     2 * time.Minute,
		RIBPeerRemovalBatchRoutes: 1000,
	}
}

// RIBMode is the mode used for the RIB
type RIBMode int

const (
	// RIBModeMemory tries to minimize used memory
	RIBModeMemory RIBMode = iota
	// RIBModePerformance keep a read-only copy of the RIB for lookups
	RIBModePerformance
)

// UnmarshalText parses a RIBMode
func (m *RIBMode) UnmarshalText(text []byte) error {
	modes := map[string]RIBMode{
		"memory":      RIBModeMemory,
		"performance": RIBModePerformance,
	}
	mode, ok := modes[strings.ToLower(string(text))]
	if !ok {
		return errors.New("unknown RIB mode")
	}
	*m = mode
	return nil
}

// String turns a RIB mode to a string
func (m RIBMode) String() string {
	modes := map[RIBMode]string{
		RIBModeMemory:      "memory",
		RIBModePerformance: "performance",
	}
	return modes[m]
}

// MarshalText turns a RIB mode to a string
func (m RIBMode) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}
