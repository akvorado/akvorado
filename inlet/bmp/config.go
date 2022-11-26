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
	// PeerRemovalMaxTime tells the maximum time the removal worker should run to remove a peer
	PeerRemovalMaxTime time.Duration `validate:"min=10ms"`
	// PeerRemovalSleepInterval tells how much time to sleep between two runs of the removal worker
	PeerRemovalSleepInterval time.Duration `validate:"min=10ms"`
	// PeerRemovalMaxQueue tells how many pending removal requests to keep
	PeerRemovalMaxQueue int `validate:"min=1"`
	// PeerRemovalMinRoutes tells how many routes we have to remove in one run before yielding
	PeerRemovalMinRoutes int `validate:"min=1"`
}

// DefaultConfiguration represents the default configuration for the BMP server
func DefaultConfiguration() Configuration {
	return Configuration{
		Listen:                   "0.0.0.0:10179",
		CollectASNs:              true,
		CollectASPaths:           true,
		CollectCommunities:       true,
		Keep:                     5 * time.Minute,
		PeerRemovalMaxTime:       200 * time.Millisecond,
		PeerRemovalSleepInterval: 500 * time.Millisecond,
		PeerRemovalMaxQueue:      10000,
		PeerRemovalMinRoutes:     5000,
	}
}
