// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package flowmodel

import (
	"net/netip"
	"testing"
)

// NewMock returns a small untrained model suitable for tests. Its
// rate profile is a flat one flow per second.
func NewMock(t *testing.T) *Model {
	t.Helper()
	return New(MockMetadata(), 0)
}

// MockMetadata returns a small metadata suitable for tests.
func MockMetadata() Metadata {
	meta := Metadata{
		Hyper: Hyperparameters{
			EmbeddingDim:   8,
			HiddenDim:      16,
			RemotePrefixes: 8,
			LocalPrefixes:  8,
			Ports:          16,
			ProtoEtypes:    4,
		},
		ProtoEtypes: []ProtoEtype{
			{EType: 0x800, Proto: 6},
			{EType: 0x800, Proto: 17},
			{EType: 0x86dd, Proto: 6},
		},
		RemotePrefixes: []RemotePrefixInfo{
			{Prefix: netip.MustParsePrefix("216.58.206.0/24"), ASN: 15169},
			{Prefix: netip.MustParsePrefix("2a00:1450:4007::/48"), ASN: 15169},
			{Prefix: netip.MustParsePrefix("198.38.120.0/23"), ASN: 2906},
			{Prefix: netip.MustParsePrefix("0.0.0.0/0")},
			{Prefix: netip.MustParsePrefix("::/0")},
		},
		LocalPrefixes: []LocalPrefixInfo{
			{Prefix: netip.MustParsePrefix("98.12.32.0/24"), HostBits: 8},
			{Prefix: netip.MustParsePrefix("3fff:100::/48"), HostBits: 10},
			{Prefix: netip.MustParsePrefix("192.0.2.0/24")},
		},
		Ports: append(DefaultPortRanges(), PortRange{Start: 443, End: 443}),
	}
	for i := range meta.RateProfile {
		meta.RateProfile[i] = 1
	}
	return meta
}
