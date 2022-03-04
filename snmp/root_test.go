package snmp

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/clock"

	"flowexporter/daemon"
	"flowexporter/helpers"
	"flowexporter/reporter"
)

func expectSNMPLookup(t *testing.T, c *Component, host string, ifIndex uint, expected Interface, expectedError error) {
	t.Helper()
	got, err := c.Lookup(host, ifIndex)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
	if !errors.Is(err, expectedError) {
		t.Errorf("Lookup() error (-got, +want):\n-%v\n+%v", err, expectedError)
	}
	if t.Failed() {
		t.FailNow()
	}
}

func TestSNMPCommunities(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	configuration.DefaultCommunity = "notpublic"
	configuration.Communities = map[string]string{
		"127.0.0.1": "public",
		"127.0.0.2": "private",
	}
	c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	defer func() {
		if err := c.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()

	// Use "public" as a community. Should work.
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{}, ErrCacheMiss)
	time.Sleep(10 * time.Millisecond)
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{
		Name:        "Gi0/0/765",
		Description: "Interface 765",
	}, nil)

	// Use "private", should not work
	expectSNMPLookup(t, c, "127.0.0.2", 765, Interface{}, ErrCacheMiss)
	time.Sleep(10 * time.Millisecond)
	expectSNMPLookup(t, c, "127.0.0.2", 765, Interface{}, ErrCacheMiss)

	// Use default community, should not work
	expectSNMPLookup(t, c, "127.0.0.3", 765, Interface{}, ErrCacheMiss)
	time.Sleep(10 * time.Millisecond)
	expectSNMPLookup(t, c, "127.0.0.3", 765, Interface{}, ErrCacheMiss)
}

func TestComponentSaveLoad(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	configuration.CachePersistFile = filepath.Join(t.TempDir(), "cache")
	c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})

	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{}, ErrCacheMiss)
	time.Sleep(10 * time.Millisecond)
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{
		Name:        "Gi0/0/765",
		Description: "Interface 765",
	}, nil)
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+c", err)
	}

	r = reporter.NewMock(t)
	c = NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{
		Name:        "Gi0/0/765",
		Description: "Interface 765",
	}, nil)
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+c", err)
	}
}

func TestAutoRefresh(t *testing.T) {
	r := reporter.NewMock(t)
	configuration := DefaultConfiguration
	mockClock := clock.NewMock()
	c := NewMock(t, r, configuration, Dependencies{Daemon: daemon.NewMock(t), Clock: mockClock})

	// Fetch a value
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{}, ErrCacheMiss)
	time.Sleep(10 * time.Millisecond)
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{
		Name:        "Gi0/0/765",
		Description: "Interface 765",
	}, nil)

	// Go forward, we expect the entry to have been refreshed and be still present
	mockClock.Add(56 * time.Minute)
	time.Sleep(10 * time.Millisecond)
	expectSNMPLookup(t, c, "127.0.0.1", 765, Interface{
		Name:        "Gi0/0/765",
		Description: "Interface 765",
	}, nil)

	// Stop and look at the cache
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}

	gotMetrics := r.GetMetrics("flowexporter_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`:      "0",
		`hit`:          "2",
		`miss`:         "1",
		`size`:         "1",
		`hosts`:        "1",
		`refresh_runs`: "28", // 56/2
		`refresh`:      "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// In 50 minutes, nothing should expire
	mockClock.Add(50 * time.Minute)
	if diff := helpers.Diff(c.sc.WouldExpire(time.Hour), map[string]map[uint]Interface{}); diff != "" {
		t.Fatalf("WouldExpire() (-got, +want):\n%s", diff)
	}

	// In 65 minutes, the entry should expire
	mockClock.Add(15 * time.Minute)
	if diff := helpers.Diff(c.sc.WouldExpire(time.Hour), map[string]map[uint]Interface{
		"127.0.0.1": {
			765: {
				Name:        "Gi0/0/765",
				Description: "Interface 765",
			},
		},
	}); diff != "" {
		t.Fatalf("WouldExpire() (-got, +want):\n%s", diff)
	}
}
