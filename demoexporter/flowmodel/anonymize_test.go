// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"net/netip"
	"testing"

	"akvorado/common/helpers"
)

func TestAnonymizePrefix(t *testing.T) {
	target4 := netip.MustParsePrefix("98.0.0.0/8")
	target6 := netip.MustParsePrefix("3fff::/20")

	t.Run("ipv4", func(t *testing.T) {
		p := netip.MustParsePrefix("203.0.113.0/24")
		got, err := AnonymizePrefix(p, target4)
		if err != nil {
			t.Fatalf("AnonymizePrefix() error:\n%+v", err)
		}
		if !target4.Contains(got.Addr()) {
			t.Fatalf("AnonymizePrefix() == %s, not in %s", got, target4)
		}
		if diff := helpers.Diff(got.Bits(), p.Bits()); diff != "" {
			t.Fatalf("AnonymizePrefix() prefix length (-got, +want):\n%s", diff)
		}
		if diff := helpers.Diff(got.String(), got.Masked().String()); diff != "" {
			t.Fatalf("AnonymizePrefix() has host bits set (-got, +want):\n%s", diff)
		}
	})

	t.Run("ipv6", func(t *testing.T) {
		p := netip.MustParsePrefix("2001:db8:cafe::/48")
		got, err := AnonymizePrefix(p, target6)
		if err != nil {
			t.Fatalf("AnonymizePrefix() error:\n%+v", err)
		}
		if !target6.Contains(got.Addr()) {
			t.Fatalf("AnonymizePrefix() == %s, not in %s", got, target6)
		}
		if diff := helpers.Diff(got.Bits(), 48); diff != "" {
			t.Fatalf("AnonymizePrefix() prefix length (-got, +want):\n%s", diff)
		}
	})

	t.Run("random", func(t *testing.T) {
		p := netip.MustParsePrefix("203.0.113.0/24")
		seen := map[netip.Prefix]bool{}
		for range 10 {
			got, err := AnonymizePrefix(p, target4)
			if err != nil {
				t.Fatalf("AnonymizePrefix() error:\n%+v", err)
			}
			seen[got] = true
		}
		if len(seen) < 2 {
			t.Fatalf("AnonymizePrefix() always gives %v", seen)
		}
	})

	t.Run("family mismatch", func(t *testing.T) {
		p := netip.MustParsePrefix("2001:db8::/32")
		if _, err := AnonymizePrefix(p, target4); err == nil {
			t.Fatalf("AnonymizePrefix() did not error on family mismatch")
		}
	})

	t.Run("target too long", func(t *testing.T) {
		p := netip.MustParsePrefix("203.0.113.0/8")
		if _, err := AnonymizePrefix(p, target4); err == nil {
			t.Fatalf("AnonymizePrefix() did not error on a target as long as the prefix")
		}
	})
}
