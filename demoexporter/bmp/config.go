// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Configuration describes the configuration for the BMP component. Only one peer is emulated.
type Configuration struct {
	// Target specify the IP address and port to generate BMP routes to. Empty if this component is disabled.
	Target string `validate:"isdefault|hostname_port"`
	// Routes is the set of routes to announce to the collector using BMP.
	Routes []RouteConfiguration `validate:"dive"`
	// LocalASN is the local AS number
	LocalASN uint16 `validate:"required,min=1"`
	// PeerASN is the peer AS number
	PeerASN uint16 `validate:"required,min=1"`
	// LocalIP is the local IP address.
	LocalIP netip.Addr `validate:"required"`
	// PeerIP is the peer IP address.
	PeerIP netip.Addr `validate:"required"`
	// RetryAfter tells how much time to wait before retrying
	RetryAfter time.Duration `validate:"min=0s"`
	// StatsDelay tells how much time to wait between two BMP stats message (to check connection liveness)
	StatsDelay time.Duration `validate:"min=0s"`
}

// RouteConfiguration describes a route to be generated with BMP.
type RouteConfiguration struct {
	// Prefix is the prefix to announce.
	Prefixes []netip.Prefix `validate:"min=1"`
	// ASPath is the AS path to associate with the prefix.
	ASPath []uint32 `validate:"min=1"`
	// Communities are the set of standard communities to associate with the prefix
	Communities []Community
}

// DefaultConfiguration represents the default configuration for the BMP component.
func DefaultConfiguration() Configuration {
	return Configuration{
		LocalASN:   64496,
		PeerASN:    64497,
		LocalIP:    netip.MustParseAddr("2001:db8::1"),
		PeerIP:     netip.MustParseAddr("2001:db8::2"),
		RetryAfter: time.Duration(5 * time.Second),
		StatsDelay: time.Duration(time.Minute),
	}
}

// Community is a standard community.
type Community uint32

// UnmarshalText parses a standard community.
func (comm *Community) UnmarshalText(input []byte) error {
	text := string(input)
	elems := strings.Split(text, ":")
	if len(elems) != 2 {
		return errors.New("community should be ASN:XX")
	}
	asn, err := strconv.ParseUint(elems[0], 10, 16)
	if err != nil {
		return errors.New("community should be ASN2:XX")
	}
	local, err := strconv.ParseUint(elems[1], 10, 16)
	if err != nil {
		return errors.New("community should be ASN:XX2")
	}
	*comm = Community((asn << 16) + local)
	return nil
}

// String turns a community to a string.
func (comm Community) String() string {
	return fmt.Sprintf("%d:%d", comm>>16, comm&0xffff)
}
