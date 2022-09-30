// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp_test

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/inlet/bmp"

	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

func TestParseRouteDistinguisher(t *testing.T) {
	cases := []struct {
		RD          string
		Expected    uint64
		CanonicalRD string // empty when same as RD
		Error       bool
	}{
		{"0", 0, "0:0", false},
		{"100", 100, "0:100", false},
		{"51324:65201", 220434901565105, "", false},
		{"51324:65536", 220434901565440, "", false},
		{"65535:0", 281470681743360, "", false},
		{"0:65535:0", 281470681743360, "65535:0", false},
		{"65536:0", 562954248388608, "", false},
		{"65536:3", 562954248388611, "", false},
		{"2:65535:0", 562954248323072, "", false},
		{"1.1.1.1:0", 282578800148480, "", false},
		{"1:1.1.1.1:0", 282578800148480, "1.1.1.1:0", false},
		{"1:1.1.1.1:0", 282578800148480, "1.1.1.1:0", false},

		{RD: "gfjkgjkf", Error: true},
		{RD: "18446744073709551616", Error: true},
		{RD: "65536:65536", Error: true},
		{RD: "0:65536:0", Error: true},
		{RD: "2:65536:65536", Error: true},
		{RD: "1:1897:0", Error: true},
		{RD: "2:1897:65536", Error: true},
		{RD: "2:1.1.1.1:0", Error: true},
		{RD: "0:1.1.1.1:0", Error: true},
	}
	for _, tc := range cases {
		if tc.CanonicalRD == "" {
			tc.CanonicalRD = tc.RD
		}
		var got bmp.RD
		err := got.UnmarshalText([]byte(tc.RD))
		if err != nil && !tc.Error {
			t.Errorf("UnmarshalText(%q) error:\n%+v", tc.RD, err)
		} else if err == nil && tc.Error {
			t.Errorf("UnmarshalText(%q) no error", tc.RD)
		} else if err != nil && tc.Error {
			continue
		} else if diff := helpers.Diff(uint64(got), tc.Expected); diff != "" {
			t.Errorf("UnmarshalText(%q) (-got, +want):\n%s", tc.RD, diff)
		} else if diff := helpers.Diff(got.String(), tc.CanonicalRD); diff != "" {
			t.Errorf("UnmarshalText(%q) (-got, +want):\n%s", tc.RD, diff)
		}
	}
}

func TestRDFromRouteDistinguisherInterface(t *testing.T) {
	cases := []struct {
		input    bgp.RouteDistinguisherInterface
		expected string
	}{
		{bgp.NewRouteDistinguisherFourOctetAS(100, 200), "2:100:200"},
		{bgp.NewRouteDistinguisherFourOctetAS(66000, 200), "66000:200"},
		{bgp.NewRouteDistinguisherTwoOctetAS(120, 200), "120:200"},
		{bgp.NewRouteDistinguisherIPAddressAS("2.2.2.2", 30), "2.2.2.2:30"},
	}
	for _, tc := range cases {
		got := bmp.RDFromRouteDistinguisherInterface(tc.input).String()
		if got != tc.expected {
			t.Errorf("RDFromRouteDistinguisherInterface(%q) == %q != %q",
				tc.input.String(), got, tc.expected)
		}
	}
}
