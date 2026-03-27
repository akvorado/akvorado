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
	// RDs list the RDs to keep. If none are specified, all received routes are
	// processed. 0 matches an absence of RD.
	RDs []RD
	// RTs list the RTs to keep. If none are specified, all received routes are
	// processed. 0 matches an absence of RT.
	RTs []RT
	// CollectASNs is true when we want to collect origin AS numbers
	CollectASNs bool
	// CollectASPaths is true when we want to collect AS paths
	CollectASPaths bool
	// CollectCommunities is true when we want to collect communities
	CollectCommunities bool
	// Keep tells how long to keep routes from a BMP client when it goes down
	Keep time.Duration `validate:"min=1s"`
	// ReceiveBuffer is the value of the requested buffer size for each
	// receiving buffer in the kernel. When 0, the value is left to the default
	// value set by the kernel (net.ipv4.tcp_rmem[1]). The value cannot exceed
	// the kernel max value (net.core.rmem_max, net.ipv4.tcp_rmem[2]).
	ReceiveBuffer uint
	// MessageBuffer is the maximum number of BMP messages buffered between the
	// TCP reader and the message processor.
	MessageBuffer uint `validate:"min=1"`
	// RIBShards is the number of shards for the RIB. Each shard has its own
	// lock, enabling concurrent route operations on different shards. The
	// maximum value matches shardBits constant in rib.go.
	RIBShards uint `validate:"oneof=1 2 4 8 16 32 64 128 256"`
}

// DefaultConfiguration represents the default configuration for the BMP server
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		Listen:             ":10179",
		CollectASNs:        true,
		CollectASPaths:     true,
		CollectCommunities: true,
		Keep:               5 * time.Minute,
		MessageBuffer:      10000,
		RIBShards:          16,
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
