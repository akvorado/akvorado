// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestConfigurationDecode(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "complete",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return map[string]any{
					"start":    "2026-08-01T00:00:00Z",
					"end":      "2026-08-08T00:00:00Z",
					"fraction": 0.25,
					"anonymization": map[string]any{
						"ipv4-prefix": "98.0.0.0/8",
						"ipv6-prefix": "3fff::/20",
					},
					"exclude-asns":     []uint32{64501},
					"exclude-prefixes": []string{"192.0.2.0/24"},
					"min-count":        10,
					"exporters":        []string{"router1"},
				}
			},
			Expected: func() Configuration {
				expected := DefaultConfiguration()
				expected.Start = start
				expected.End = end
				expected.Fraction = 0.25
				expected.Anonymization = AnonymizationConfiguration{
					IPv4Prefix: netip.MustParsePrefix("98.0.0.0/8"),
					IPv6Prefix: netip.MustParsePrefix("3fff::/20"),
				}
				expected.ExcludeASNs = []uint32{64501}
				expected.ExcludePrefixes = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
				expected.MinCount = 10
				expected.Exporters = []string{"router1"}
				return expected
			}(),
		}, {
			Description: "end before start",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return map[string]any{
					"start": "2026-08-08T00:00:00Z",
					"end":   "2026-08-01T00:00:00Z",
				}
			},
			Error: true,
		}, {
			Description: "zero min count",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return map[string]any{
					"start":     "2026-08-01T00:00:00Z",
					"end":       "2026-08-08T00:00:00Z",
					"min-count": 0,
				}
			},
			Error: true,
		},
	})
}

func TestCheck(t *testing.T) {
	config := DefaultConfiguration()
	if err := config.Check(); err == nil {
		t.Fatalf("Check() did not error without target prefixes")
	}
	config.Anonymization.IPv4Prefix = netip.MustParsePrefix("3fff::/20")
	config.Anonymization.IPv6Prefix = netip.MustParsePrefix("3fff::/20")
	if err := config.Check(); err == nil {
		t.Fatalf("Check() did not error with an IPv6 prefix as the IPv4 target prefix")
	}
	config.Anonymization.IPv4Prefix = netip.MustParsePrefix("98.0.0.0/8")
	config.Anonymization.IPv6Prefix = netip.MustParsePrefix("::ffff:98.0.0.0/104")
	if err := config.Check(); err == nil {
		t.Fatalf("Check() did not error with an IPv4 prefix as the IPv6 target prefix")
	}
	config.Anonymization.IPv6Prefix = netip.MustParsePrefix("3fff::/20")
	if err := config.Check(); err != nil {
		t.Fatalf("Check() error:\n%+v", err)
	}
}

func TestLearnRemote(t *testing.T) {
	config := DefaultConfiguration()
	locals := []netip.Prefix{netip.MustParsePrefix("100.0.0.0/16")}
	config.ExcludeASNs = []uint32{3215}
	config.ExcludePrefixes = []netip.Prefix{netip.MustParsePrefix("90.0.0.0/8")}
	cases := []struct {
		Prefix   string
		ASN      uint32
		Expected bool
	}{
		{"216.58.206.0/24", 15169, true},
		{"2a00:1450::/32", 15169, true},
		{"0.0.0.0/0", 15169, false},
		{"216.58.206.0/25", 15169, false},
		{"2a00:1450::/56", 15169, false},
		{"216.58.206.0/24", 0, false},
		{"216.58.206.0/24", 64512, false},
		{"216.58.206.0/24", 3215, false},
		{"90.10.0.0/16", 15169, false},
		{"100.0.1.0/24", 15169, false},
		{"10.1.0.0/16", 15169, false},
		{"100.64.0.0/24", 15169, false},
		{"fd00::/48", 15169, false},
		{"2001:db8::/32", 15169, false},
	}
	for _, tc := range cases {
		got := config.learnRemote(netip.MustParsePrefix(tc.Prefix), tc.ASN, locals)
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("learnRemote(%s, %d) (-got, +want):\n%s", tc.Prefix, tc.ASN, diff)
		}
	}
}
