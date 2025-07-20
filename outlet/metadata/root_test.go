// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import (
	"context"
	"errors"
	"net/netip"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/clock"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/outlet/metadata/provider"
	"akvorado/outlet/metadata/provider/static"
)

func expectMockLookup(t *testing.T, c *Component, exporter string, ifIndex uint, expected provider.Answer) {
	t.Helper()
	ip := netip.AddrFrom16(netip.MustParseAddr(exporter).As16())
	got, _ := c.Lookup(time.Now(), ip, ifIndex)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}
}

func TestLookup(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration(), Dependencies{Daemon: daemon.NewMock(t)})
	expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{})
	expectMockLookup(t, c, "127.0.0.1", 999, provider.Answer{})
	time.Sleep(30 * time.Millisecond)
	expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{
		Exporter: provider.Exporter{
			Name: "127_0_0_1",
		},

		Interface: provider.Interface{Name: "Gi0/0/765",
			Description: "Interface 765",
			Speed:       1000,
		},
	})
	expectMockLookup(t, c, "127.0.0.1", 999, provider.Answer{
		Exporter: provider.Exporter{
			Name: "127_0_0_1",
		},
	})
}

func TestComponentSaveLoad(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.CachePersistFile = filepath.Join(t.TempDir(), "cache")

	t.Run("save", func(t *testing.T) {
		r := reporter.NewMock(t)
		c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})

		expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{})
		time.Sleep(30 * time.Millisecond)
		expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{
			Exporter: provider.Exporter{
				Name: "127_0_0_1",
			},

			Interface: provider.Interface{Name: "Gi0/0/765",
				Description: "Interface 765",
				Speed:       1000,
			},
		})
	})

	t.Run("load", func(t *testing.T) {
		r := reporter.NewMock(t)
		c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
		expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{
			Exporter: provider.Exporter{
				Name: "127_0_0_1",
			},
			Interface: provider.Interface{
				Name:        "Gi0/0/765",
				Description: "Interface 765",
				Speed:       1000,
			},
		})
	})
}

func TestAutoRefresh(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration()
	mockClock := clock.NewMock()
	c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t), Clock: mockClock})

	// Fetch a value
	expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{})
	time.Sleep(30 * time.Millisecond)
	expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{
		Exporter: provider.Exporter{
			Name: "127_0_0_1",
		},

		Interface: provider.Interface{
			Name:        "Gi0/0/765",
			Description: "Interface 765",
			Speed:       1000,
		},
	})

	// Keep it in the cache!
	mockClock.Add(25 * time.Minute)
	c.Lookup(mockClock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 765)
	mockClock.Add(25 * time.Minute)
	c.Lookup(mockClock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 765)

	// Go forward, we expect the entry to have been refreshed and be still present
	mockClock.Add(11 * time.Minute)
	time.Sleep(30 * time.Millisecond)
	mockClock.Add(2 * time.Minute)
	time.Sleep(30 * time.Millisecond)
	expectMockLookup(t, c, "127.0.0.1", 765, provider.Answer{
		Exporter: provider.Exporter{
			Name: "127_0_0_1",
		},

		Interface: provider.Interface{Name: "Gi0/0/765",
			Description: "Interface 765",
			Speed:       1000,
		},
	})

	gotMetrics := r.GetMetrics("akvorado_outlet_metadata_cache_")
	for _, runs := range []string{"29", "30", "31"} { // 63/2
		expectedMetrics := map[string]string{
			`expired_entries_total`: "0",
			`hits_total`:            "4",
			`misses_total`:          "1",
			`size_entries`:          "1",
			`refresh_runs_total`:    runs,
			`refreshs_total`:        "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" && runs == "31" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		} else if diff == "" {
			break
		}
	}
}

func TestConfigCheck(t *testing.T) {
	t.Run("refresh", func(t *testing.T) {
		configuration := DefaultConfiguration()
		configuration.CacheDuration = 10 * time.Minute
		configuration.CacheRefresh = 5 * time.Minute
		configuration.CacheCheckInterval = time.Minute
		configuration.Providers = []ProviderConfiguration{{Config: mockProviderConfiguration{}}}
		if _, err := New(reporter.NewMock(t), configuration, Dependencies{Daemon: daemon.NewMock(t)}); err == nil {
			t.Fatal("New() should trigger an error")
		}
	})
	t.Run("interval", func(t *testing.T) {
		configuration := DefaultConfiguration()
		configuration.CacheDuration = 10 * time.Minute
		configuration.CacheRefresh = 15 * time.Minute
		configuration.CacheCheckInterval = 12 * time.Minute
		configuration.Providers = []ProviderConfiguration{{Config: mockProviderConfiguration{}}}
		if _, err := New(reporter.NewMock(t), configuration, Dependencies{Daemon: daemon.NewMock(t)}); err == nil {
			t.Fatal("New() should trigger an error")
		}
	})
	t.Run("refresh disabled", func(t *testing.T) {
		configuration := DefaultConfiguration()
		configuration.CacheDuration = 10 * time.Minute
		configuration.CacheRefresh = 0
		configuration.CacheCheckInterval = 2 * time.Minute
		configuration.Providers = []ProviderConfiguration{{Config: mockProviderConfiguration{}}}
		if _, err := New(reporter.NewMock(t), configuration, Dependencies{Daemon: daemon.NewMock(t)}); err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
	})
}

func TestStartStopWithMultipleWorkers(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration()
	configuration.Workers = 5
	NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
}

type errorProvider struct{}

func (ep errorProvider) Query(_ context.Context, _ *provider.BatchQuery) error {
	return errors.New("noooo")
}

type errorProviderConfiguration struct{}

func (epc errorProviderConfiguration) New(_ *reporter.Reporter, _ func(provider.Update)) (provider.Provider, error) {
	return errorProvider{}, nil
}

func TestProviderBreaker(t *testing.T) {
	cases := []struct {
		Name                  string
		ProviderConfiguration provider.Configuration
		ExpectedCount         string
	}{
		{"always successful provider", mockProviderConfiguration{}, "0"},
		{"never successful provider", errorProviderConfiguration{}, "10"},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration()
			configuration.MaxBatchRequests = 0
			configuration.Providers = []ProviderConfiguration{{Config: tc.ProviderConfiguration}}
			c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
			c.metrics.providerBreakerOpenCount.WithLabelValues("127.0.0.1").Add(0)

			for range 30 {
				c.Lookup(c.d.Clock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 765)
			}
			for range 5 {
				c.Lookup(c.d.Clock.Now(), netip.MustParseAddr("::ffff:127.0.0.2"), 765)
			}
			time.Sleep(50 * time.Millisecond)

			gotMetrics := r.GetMetrics("akvorado_outlet_metadata_provider_", "breaker_opens_total")
			expectedMetrics := map[string]string{
				`breaker_opens_total{exporter="127.0.0.1"}`: tc.ExpectedCount,
			}
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				t.Errorf("Metrics (-got, +want):\n%s", diff)
			}
		})
	}
}

type batchProvider struct {
	config *batchProviderConfiguration
}

func (bp *batchProvider) Query(_ context.Context, query *provider.BatchQuery) error {
	bp.config.received = append(bp.config.received, *query)
	return nil
}

type batchProviderConfiguration struct {
	received []provider.BatchQuery
}

func (bpc *batchProviderConfiguration) New(_ *reporter.Reporter, _ func(provider.Update)) (provider.Provider, error) {
	return &batchProvider{config: bpc}, nil
}

func TestBatching(t *testing.T) {
	bcp := batchProviderConfiguration{
		received: []provider.BatchQuery{},
	}
	r := reporter.NewMock(t)
	t.Run("run", func(t *testing.T) {
		configuration := DefaultConfiguration()
		configuration.Providers = []ProviderConfiguration{{Config: &bcp}}
		c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})

		// Block dispatcher
		blocker := make(chan bool)
		c.dispatcherBChannel <- blocker

		defer func() {
			// Unblock
			time.Sleep(20 * time.Millisecond)
			close(blocker)
			time.Sleep(20 * time.Millisecond)
		}()

		// Queue requests
		c.Lookup(c.d.Clock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 766)
		c.Lookup(c.d.Clock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 767)
		c.Lookup(c.d.Clock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 768)
		c.Lookup(c.d.Clock.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), 769)
	})

	t.Run("check", func(t *testing.T) {
		gotMetrics := r.GetMetrics("akvorado_outlet_metadata_provider_", "batched_requests_total")
		expectedMetrics := map[string]string{
			`batched_requests_total`: "4",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedAccepted := []provider.BatchQuery{
			{
				ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
				IfIndexes:  []uint{766, 767, 768, 769},
			},
		}
		if diff := helpers.Diff(bcp.received, expectedAccepted); diff != "" {
			t.Errorf("Accepted requests (-got, +want):\n%s", diff)
		}
	})
}

func TestMultipleProviders(t *testing.T) {
	r := reporter.NewMock(t)
	staticConfiguration1 := static.Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]static.ExporterConfiguration{
			"2001:db8:1::/48": {
				Exporter: provider.Exporter{
					Name: "static1",
				},
				IfIndexes: map[uint]provider.Interface{
					10: {
						Name:        "Gi10",
						Description: "10th interface",
						Speed:       1000,
					},
					11: {
						Name:        "Gi11",
						Description: "11th interface",
						Speed:       1000,
					},
				},
			},
		}),
	}
	staticConfiguration2 := static.Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]static.ExporterConfiguration{
			"2001:db8:2::/48": {
				Exporter: provider.Exporter{
					Name: "static2",
				},
				IfIndexes: map[uint]provider.Interface{
					12: {
						Name:        "Gi12",
						Description: "12th interface",
						Speed:       1000,
					},
					13: {
						Name:        "Gi13",
						Description: "13th interface",
						Speed:       1000,
					},
				},
			},
		}),
	}
	configuration := DefaultConfiguration()
	configuration.Providers = []ProviderConfiguration{
		{Config: staticConfiguration1},
		{Config: staticConfiguration2},
	}
	c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	c.Lookup(time.Now(), netip.MustParseAddr("2001:db8:1::1"), 10)
	c.Lookup(time.Now(), netip.MustParseAddr("2001:db8:2::2"), 12)
	time.Sleep(30 * time.Millisecond)
	got1, _ := c.Lookup(time.Now(), netip.MustParseAddr("2001:db8:1::1"), 10)
	got2, _ := c.Lookup(time.Now(), netip.MustParseAddr("2001:db8:2::2"), 12)
	got := []provider.Answer{got1, got2}
	expected := []provider.Answer{
		{
			Exporter: provider.Exporter{
				Name: "static1",
			},
			Interface: provider.Interface{
				Name:        "Gi10",
				Description: "10th interface",
				Speed:       1000,
			},
		}, {
			Exporter: provider.Exporter{
				Name: "static2",
			},
			Interface: provider.Interface{
				Name:        "Gi12",
				Description: "12th interface",
				Speed:       1000,
			},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("Lookup() (-got, +want):\n%s", diff)
	}
}
