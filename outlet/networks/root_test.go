// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/common/reporter"
)

// amazonJSON mimics the network list published by Amazon.
const amazonJSON = `
{
  "syncToken": "1665609189",
  "createDate": "2022-10-12-21-13-09",
  "prefixes": [
    {
      "ip_prefix": "3.2.34.0/26",
      "region": "af-south-1",
      "service": "AMAZON",
      "network_border_group": "af-south-1"
    }
  ],
  "ipv6_prefixes": [
    {
      "ipv6_prefix": "2600:1ff2:4000::/40",
      "region": "us-west-2",
      "service": "AMAZON",
      "network_border_group": "us-west-2"
    },
    {
      "ipv6_prefix": "2600:1f14:fff:f800::/56",
      "region": "us-west-2",
      "service": "ROUTE53_HEALTHCHECKS",
      "network_border_group": "us-west-2"
    }
  ]
}
`

// amazonSource returns a network source fetching the provided URL.
func amazonSource(url string) map[string]remotedatasource.Source {
	return map[string]remotedatasource.Source{
		"amazon": {
			URL:      url,
			Method:   "GET",
			Timeout:  time.Second,
			Interval: time.Minute,
			Transform: remotedatasource.MustParseTransformQuery(`
(.prefixes + .ipv6_prefixes)[] |
{ prefix: (.ip_prefix // .ipv6_prefix), tenant: "amazon", region: .region, role: .service|ascii_downcase }
`),
		},
	}
}

func TestLookupStaticNetworks(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:192.0.2.0/120":    {Name: "customer1", Role: "customer", Site: "paris", Region: "eu-west", Tenant: "mobile"},
		"::ffff:198.51.100.0/120": {Name: "infra", Role: "server"},
	})
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	cases := []struct {
		description string
		ip          string
		expected    NetworkAttributes
	}{
		{
			description: "matching first network",
			ip:          "::ffff:192.0.2.1",
			expected:    NetworkAttributes{Name: "customer1", Role: "customer", Site: "paris", Region: "eu-west", Tenant: "mobile"},
		},
		{
			description: "matching second network",
			ip:          "::ffff:198.51.100.5",
			expected:    NetworkAttributes{Name: "infra", Role: "server"},
		},
		{
			description: "no match",
			ip:          "::ffff:203.0.113.1",
			expected:    NetworkAttributes{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got := c.Lookup(netip.MustParseAddr(tc.ip))
			if diff := helpers.Diff(got, tc.expected); diff != "" {
				t.Fatalf("Lookup(%q) (-got, +want):\n%s", tc.ip, diff)
			}
		})
	}
}

func TestLookupHierarchicalInheritance(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:10.0.0.0/104": {Region: "eu-west", Tenant: "corp"},
		"::ffff:10.1.0.0/112": {Name: "office", Site: "paris"},
		"::ffff:10.1.1.0/120": {Role: "server"},
	})
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// 10.1.1.1 matches all three prefixes; should inherit from parent
	got := c.Lookup(netip.MustParseAddr("::ffff:10.1.1.1"))
	expected := NetworkAttributes{
		Name:   "office",
		Role:   "server",
		Site:   "paris",
		Region: "eu-west",
		Tenant: "corp",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}

	// 10.1.2.1 matches first two prefixes
	got = c.Lookup(netip.MustParseAddr("::ffff:10.1.2.1"))
	expected = NetworkAttributes{
		Name:   "office",
		Site:   "paris",
		Region: "eu-west",
		Tenant: "corp",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}

	// 10.2.0.1 matches only the /104
	got = c.Lookup(netip.MustParseAddr("::ffff:10.2.0.1"))
	expected = NetworkAttributes{
		Region: "eu-west",
		Tenant: "corp",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}
}

func TestLookupWithASN(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:192.0.2.0/120": {Name: "customer1", ASN: 65001},
	})
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	got := c.Lookup(netip.MustParseAddr("::ffff:192.0.2.1"))
	expected := NetworkAttributes{Name: "customer1", ASN: 65001}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}
}

func TestLookupNetworkSources(t *testing.T) {
	r := reporter.NewMock(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(amazonJSON))
	}))
	defer server.Close()

	config := DefaultConfiguration()
	config.NetworkSources = amazonSource(server.URL)
	// The static networks take precedence over the remote ones.
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:3.2.34.0/122":  {Role: "servers"},
		"::ffff:192.0.2.0/120": {Name: "customer1"},
	})

	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	cases := []struct {
		description string
		ip          string
		expected    NetworkAttributes
	}{
		{
			description: "remote network, role overridden by the static networks",
			ip:          "::ffff:3.2.34.1",
			expected:    NetworkAttributes{Role: "servers", Region: "af-south-1", Tenant: "amazon"},
		}, {
			description: "remote IPv6 network",
			ip:          "2600:1f14:fff:f800::1",
			expected:    NetworkAttributes{Role: "route53_healthchecks", Region: "us-west-2", Tenant: "amazon"},
		}, {
			description: "remote IPv6 network, less specific",
			ip:          "2600:1ff2:4000::1",
			expected:    NetworkAttributes{Role: "amazon", Region: "us-west-2", Tenant: "amazon"},
		}, {
			description: "static network only",
			ip:          "::ffff:192.0.2.1",
			expected:    NetworkAttributes{Name: "customer1"},
		}, {
			description: "no match",
			ip:          "::ffff:203.0.113.1",
			expected:    NetworkAttributes{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got := c.Lookup(netip.MustParseAddr(tc.ip))
			if diff := helpers.Diff(got, tc.expected); diff != "" {
				t.Fatalf("Lookup(%q) (-got, +want):\n%s", tc.ip, diff)
			}
		})
	}

	gotMetrics := r.GetMetrics("akvorado_common_remotedatasource_data_")
	expectedMetrics := map[string]string{
		`total{source="amazon",type="network_source"}`: "3",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestLookupNetworkSourcesNotReady(t *testing.T) {
	r := reporter.NewMock(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := DefaultConfiguration()
	config.NetworkSources = amazonSource(server.URL)
	config.NetworkSourcesTimeout = 100 * time.Millisecond
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:192.0.2.0/120": {Name: "customer1"},
	})

	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	// Start does not block longer than the timeout and the static networks are
	// used in the meantime.
	helpers.StartStop(t, c)

	if diff := helpers.Diff(c.Lookup(netip.MustParseAddr("::ffff:192.0.2.1")),
		NetworkAttributes{Name: "customer1"}); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(c.Lookup(netip.MustParseAddr("::ffff:3.2.34.1")),
		NetworkAttributes{}); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
}
