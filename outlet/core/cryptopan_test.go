// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/base64"
	"net"
	"testing"
)

// Basic determinism & prefix-preservation test for Crypto-PAn via Anonymizer.
func TestAnonymizerDeterminismAndPrefixPreservation(t *testing.T) {
	// deterministic key for test
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	keyStr := base64.StdEncoding.EncodeToString(key)

	cfg := AnonymizeConfig{
		Enabled: true,
		Mode:    AnonymizeModeCryptoPan,
		CryptoPan: AnonymizeCryptoPanConfig{
			Key:   keyStr,
			Cache: 1024,
		},
		Aggregate: AnonymizeAggregateConfig{
			V4Prefix: 24,
			V6Prefix: 64,
		},
	}
	a, err := NewAnonymizer(cfg)
	if err != nil {
		t.Fatalf("NewAnonymizer: %v", err)
	}
	if !a.enabled {
		t.Skip("anonymizer disabled")
	}

	// IPv4 determinism
	ipv4 := net.ParseIP("10.1.2.3")
	a1 := a.AnonymizeIP(ipv4)
	a2 := a.AnonymizeIP(ipv4)
	if !a1.Equal(a2) {
		t.Fatalf("IPv4 anonymization not deterministic: %v != %v", a1, a2)
	}

	// IPv4 prefix preservation: pick two ips sharing /16
	ip4a := net.ParseIP("10.1.2.3")
	ip4b := net.ParseIP("10.1.3.4")
	aa := a.AnonymizeIP(ip4a).To4()
	ab := a.AnonymizeIP(ip4b).To4()
	if aa == nil || ab == nil {
		t.Fatalf("expected IPv4 addresses")
	}
	// check first 2 bytes (16 bits) equal
	if aa[0] != ab[0] || aa[1] != ab[1] {
		t.Fatalf("IPv4 prefix not preserved: %v vs %v", aa, ab)
	}

	// IPv6 determinism
	ip6 := net.ParseIP("2001:db8:1::1")
	b1 := a.AnonymizeIP(ip6)
	b2 := a.AnonymizeIP(ip6)
	if !b1.Equal(b2) {
		t.Fatalf("IPv6 anonymization not deterministic: %v != %v", b1, b2)
	}

	// IPv6 prefix preservation: pick two ips sharing at least /64
	ip6a := net.ParseIP("2001:db8:1::1")
	ip6b := net.ParseIP("2001:db8:1::2")
	ba := a.AnonymizeIP(ip6a).To16()
	bb := a.AnonymizeIP(ip6b).To16()
	if ba == nil || bb == nil {
		t.Fatalf("expected IPv6 addresses")
	}
	// check first 8 bytes (64 bits) equal
	for i := 0; i < 8; i++ {
		if ba[i] != bb[i] {
			t.Fatalf("IPv6 prefix not preserved at byte %d: %x vs %x", i, ba[i], bb[i])
		}
	}
}
