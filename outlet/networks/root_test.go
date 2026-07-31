// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package networks

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/common/reporter"
	"akvorado/outlet/geoip"
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

func TestLookupDeepHierarchicalInheritance(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		"::ffff:10.0.0.0/104":   {Tenant: "corp"},
		"::ffff:10.1.0.0/112":   {Region: "eu-west"},
		"::ffff:10.1.1.0/120":   {Site: "paris"},
		"::ffff:10.1.1.128/121": {Role: "server"},
		// Leaving the branch above, only the /104 is inherited
		"::ffff:10.2.0.0/112": {Name: "other"},
		// Leaving the /104 altogether, nothing is inherited
		"::ffff:192.0.2.0/120": {Name: "elsewhere"},
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
			description: "four levels of inheritance",
			ip:          "::ffff:10.1.1.129",
			expected: NetworkAttributes{
				Tenant: "corp", Region: "eu-west", Site: "paris", Role: "server",
			},
		}, {
			description: "three levels of inheritance",
			ip:          "::ffff:10.1.1.1",
			expected: NetworkAttributes{
				Tenant: "corp", Region: "eu-west", Site: "paris",
			},
		}, {
			description: "sibling branch only inherits from the top prefix",
			ip:          "::ffff:10.2.0.1",
			expected:    NetworkAttributes{Tenant: "corp", Name: "other"},
		}, {
			description: "disjoint prefix inherits nothing",
			ip:          "::ffff:192.0.2.1",
			expected:    NetworkAttributes{Name: "elsewhere"},
		}, {
			description: "only the top prefix matches",
			ip:          "::ffff:10.3.0.1",
			expected:    NetworkAttributes{Tenant: "corp"},
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

func TestLookupWithGeoIP(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	config.Networks = helpers.MustNewSubnetMap(map[string]NetworkAttributes{
		// Less specific than the GeoIP prefixes covering 67.43.156.77
		"::ffff:67.0.0.0/104": {Name: "customer1", Country: "FR", City: "Nantes", ASN: 65001},
		// More specific than the GeoIP prefixes covering 213.248.218.137
		"::ffff:213.248.218.137/128": {Name: "customer2", Country: "DE", ASN: 65002},
	})
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		GeoIP:  geoip.NewMock(t, r, true),
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
			description: "GeoIP wins on the more specific prefix",
			ip:          "::ffff:67.43.156.77",
			expected: NetworkAttributes{
				Name: "customer1", Country: "BT", City: "Nantes", ASN: 35908,
			},
		}, {
			description: "static network wins on the more specific prefix",
			ip:          "::ffff:213.248.218.137",
			expected: NetworkAttributes{
				Name: "customer2", Country: "DE", ASN: 65002,
			},
		}, {
			description: "GeoIP only",
			ip:          "::ffff:2.19.4.138",
			expected:    NetworkAttributes{Country: "SG", ASN: 32787},
		}, {
			description: "no match",
			ip:          "::ffff:203.0.113.5",
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

	gotMetrics := r.GetMetrics("akvorado_outlet_networks_", "rebuilds_total")
	expectedMetrics := map[string]string{"rebuilds_total": "1"}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestRebuildOnGeoIPUpdate(t *testing.T) {
	dir := t.TempDir()
	asnFile := filepath.Join(dir, "asn.mmdb")
	geoipConfig := geoip.DefaultConfiguration()
	geoipConfig.ASNDatabase = []string{asnFile}
	geoipConfig.Optional = true

	r := reporter.NewMock(t)
	geoipComponent, err := geoip.New(r, geoipConfig, geoip.Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("geoip.New() error:\n%+v", err)
	}
	helpers.StartStop(t, geoipComponent)

	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon: daemon.NewMock(t),
		GeoIP:  geoipComponent,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// The database is not there yet
	ip := netip.MustParseAddr("::ffff:67.43.156.77")
	if diff := helpers.Diff(c.Lookup(ip), NetworkAttributes{}); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}

	// Once it appears, the networks are rebuilt with its content. The database
	// is renamed into place to not be opened while incomplete.
	source, err := os.ReadFile(geoip.TestDataPath("GeoLite2-ASN-Test.mmdb"))
	if err != nil {
		t.Fatalf("os.ReadFile() error:\n%+v", err)
	}
	if err := os.WriteFile(asnFile+".tmp", source, 0o666); err != nil {
		t.Fatalf("os.WriteFile() error:\n%+v", err)
	}
	if err := os.Rename(asnFile+".tmp", asnFile); err != nil {
		t.Fatalf("os.Rename() error:\n%+v", err)
	}
	expected := NetworkAttributes{ASN: 35908}
	var diff string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if diff = helpers.Diff(c.Lookup(ip), expected); diff == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Lookup() (-got, +want):\n%s", diff)
}

// TestUpdateSourceWithoutChange checks refetching a source whose content did
// not change does not rebuild: this happens on every refresh interval and
// walking the GeoIP databases is expensive.
func TestUpdateSourceWithoutChange(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	rebuilds := func() string {
		t.Helper()
		return r.GetMetrics("akvorado_outlet_networks_", "rebuilds_total")["rebuilds_total"]
	}
	lookup := func() NetworkAttributes {
		t.Helper()
		return c.Lookup(netip.MustParseAddr("::ffff:192.0.2.1"))
	}
	source := func(name string) []externalNetworkAttributes {
		return []externalNetworkAttributes{{
			Prefix:            netip.MustParsePrefix("::ffff:192.0.2.0/120"),
			NetworkAttributes: NetworkAttributes{Name: name},
		}}
	}

	// Only the rebuild from Start() so far
	if diff := helpers.Diff(rebuilds(), "1"); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// A new source is published
	c.updateSource("source", source("customer1"))
	if diff := helpers.Diff(rebuilds(), "2"); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(lookup(), NetworkAttributes{Name: "customer1"}); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}

	// Refetching the same content is compared by value, not by identity
	c.updateSource("source", source("customer1"))
	if diff := helpers.Diff(rebuilds(), "2"); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// A real change is still published
	c.updateSource("source", source("customer2"))
	if diff := helpers.Diff(rebuilds(), "3"); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(lookup(), NetworkAttributes{Name: "customer2"}); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}
}

// TestConcurrentRebuilds checks a rebuild skipped because another one was
// already running still published the changes of the caller which was skipped.
func TestConcurrentRebuilds(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	// Publish a source and ask for a rebuild, like UpdateSource does
	const sources = 20
	var wg sync.WaitGroup
	for i := range sources {
		wg.Go(func() {
			c.updateSource(fmt.Sprintf("source%d", i), []externalNetworkAttributes{{
				Prefix:            netip.MustParsePrefix(fmt.Sprintf("::ffff:10.%d.0.0/112", i)),
				NetworkAttributes: NetworkAttributes{Name: fmt.Sprintf("net%d", i)},
			}})
		})
	}
	wg.Wait()

	for i := range sources {
		ip := fmt.Sprintf("::ffff:10.%d.0.1", i)
		got := c.Lookup(netip.MustParseAddr(ip))
		expected := NetworkAttributes{Name: fmt.Sprintf("net%d", i)}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Lookup(%q) (-got, +want):\n%s", ip, diff)
		}
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
