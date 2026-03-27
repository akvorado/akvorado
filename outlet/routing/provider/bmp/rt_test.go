// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp_test

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/outlet/routing/provider/bmp"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

func TestParseRouteTarget(t *testing.T) {
	cases := []struct {
		RT          string
		CanonicalRT string
		Error       bool
	}{
		{"0", "0:0", false},
		{"51324:65201", "51324:65201", false},
		{"51324:65536", "51324:65536", false},
		{"65535:0", "65535:0", false},
		{"65536:0", "65536:0", false},
		{"65536:3", "65536:3", false},
		{"1.1.1.1:0", "1.1.1.1:0", false},
		{"65017:104", "65017:104", false},

		{RT: "gfjkgjkf", Error: true},
	}
	for _, tc := range cases {
		var got bmp.RT
		err := got.UnmarshalText([]byte(tc.RT))
		if err != nil && !tc.Error {
			t.Errorf("UnmarshalText(%q) error:\n%+v", tc.RT, err)
		} else if err == nil && tc.Error {
			t.Errorf("UnmarshalText(%q) no error", tc.RT)
		} else if err != nil && tc.Error {
			continue
		} else if diff := helpers.Diff(got.String(), tc.CanonicalRT); diff != "" {
			t.Errorf("UnmarshalText(%q) (-got, +want):\n%s", tc.RT, diff)
		}
	}
}

func TestRTFromExtendedCommunity(t *testing.T) {
	cases := []struct {
		description string
		input       bgp.ExtendedCommunityInterface
		expected    string
		isRT        bool
	}{
		{
			description: "2-octet AS RT",
			input:       bgp.NewTwoOctetAsSpecificExtended(bgp.EC_SUBTYPE_ROUTE_TARGET, 65017, 104, true),
			expected:    "65017:104",
			isRT:        true,
		}, {
			description: "4-octet AS RT",
			input:       bgp.NewFourOctetAsSpecificExtended(bgp.EC_SUBTYPE_ROUTE_TARGET, 100000, 200, true),
			expected:    "100000:200",
			isRT:        true,
		}, {
			description: "not a RT (route origin)",
			input:       bgp.NewTwoOctetAsSpecificExtended(bgp.EC_SUBTYPE_ROUTE_ORIGIN, 65017, 104, true),
			isRT:        false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got, ok := bmp.RTFromExtendedCommunity(tc.input)
			if !ok && tc.isRT {
				t.Fatal("RTFromExtendedCommunity() returned false, expected RT")
			} else if ok && !tc.isRT {
				t.Fatalf("RTFromExtendedCommunity() returned %s, expected false", got)
			} else if ok {
				if diff := helpers.Diff(got.String(), tc.expected); diff != "" {
					t.Fatalf("RTFromExtendedCommunity() (-got, +want):\n%s", diff)
				}
			}
		})
	}
}
