// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package provider defines the interface of a provider module for routing
// information.
package provider

import (
	"context"
	"net/netip"

	"akvorado/common/daemon"
	"akvorado/common/reporter"

	"github.com/benbjohnson/clock"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

// LookupResult is the result of the Lookup() function.
type LookupResult struct {
	ASN              uint32
	ASPath           []uint32
	Communities      []uint32
	LargeCommunities []bgp.LargeCommunity
	NetMask          uint8
}

// Dependencies are the dependencies for a provider.
type Dependencies struct {
	Daemon daemon.Component
	Clock  clock.Clock
}

// Provider is the interface a provider should implement.
type Provider interface {
	// Lookup asks the provider about information for a given IP address and
	// next-hop.
	Lookup(ctx context.Context, ip netip.Addr, nh netip.Addr) LookupResult
}

// Configuration defines an interface to configure a provider.
type Configuration interface {
	// New instantiates a new provider from its configuration.
	New(r *reporter.Reporter, d Dependencies) (Provider, error)
}
