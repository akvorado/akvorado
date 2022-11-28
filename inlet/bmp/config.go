// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import "time"

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
	// RIBPeerRemovalMaxTime tells the maximum time the removal worker should run to remove a peer
	RIBPeerRemovalMaxTime time.Duration `validate:"min=10ms"`
	// RIBPeerRemovalSleepInterval tells how much time to sleep between two runs of the removal worker
	RIBPeerRemovalSleepInterval time.Duration `validate:"min=10ms"`
	// RIBPeerRemovalMaxQueue tells how many pending removal requests to keep
	RIBPeerRemovalMaxQueue int `validate:"min=1"`
	// RIBPeerRemovalBatchRoutes tells how many routes to remove before checking
	// if we have a higher priority request. This is only if RIB is in memory
	// mode.
	RIBPeerRemovalBatchRoutes int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the BMP server
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen:                      "0.0.0.0:10179",
		CollectASNs:                 true,
		CollectASPaths:              true,
		CollectCommunities:          true,
		Keep:                        5 * time.Minute,
		RIBPeerRemovalMaxTime:       100 * time.Millisecond,
		RIBPeerRemovalSleepInterval: 500 * time.Millisecond,
		RIBPeerRemovalMaxQueue:      10000,
		RIBPeerRemovalBatchRoutes:   5000,
	}
}
