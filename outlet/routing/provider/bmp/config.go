// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"time"

	"akvorado/common/helpers"
	"akvorado/outlet/routing/provider"
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
}

// DefaultConfiguration represents the default configuration for the BMP server
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		Listen:             ":10179",
		CollectASNs:        true,
		CollectASPaths:     true,
		CollectCommunities: true,
		Keep:               5 * time.Minute,
	}
}

func init() {
	helpers.RegisterMapstructureDeprecatedFields[Configuration](
		"RIBPeerRemovalMaxTime",
		"RIBPeerRemovalSleepInterval",
		"RIBPeerRemovalMaxQueue",
		"RIBPeerRemovalBatchRoutes",
	)
}
