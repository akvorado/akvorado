// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package routing

import (
	"context"
	"net/netip"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestRoutingComponent(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r)
	helpers.StartStop(t, c)
	c.PopulateRIB(t)

	lookup := c.Lookup(context.Background(),
		netip.MustParseAddr("::ffff:192.0.2.2"),
		netip.MustParseAddr("::ffff:198.51.100.200"))
	if lookup.ASN != 174 {
		t.Errorf("Lookup() == %d, expected 174", lookup.ASN)
	}
	lookup = c.Lookup(context.Background(),
		netip.MustParseAddr("::ffff:192.0.2.254"),
		netip.MustParseAddr("::ffff:198.51.100.200"))
	if lookup.ASN != 0 {
		t.Errorf("Lookup() == %d, expected 0", lookup.ASN)
	}
}
