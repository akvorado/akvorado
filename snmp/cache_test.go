package snmp

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/clock"

	"akvorado/helpers"
	"akvorado/reporter"
)

func setupTestCache(t *testing.T) (*reporter.Reporter, *clock.Mock, *snmpCache) {
	t.Helper()
	r := reporter.NewMock(t)
	clock := clock.NewMock()
	sc := newSNMPCache(r, clock)
	return r, clock, sc
}

func expectCacheLookup(t *testing.T, sc *snmpCache, host string, ifIndex uint, expected Interface, expectedError error) {
	t.Helper()
	got, err := sc.Lookup(host, ifIndex)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
	if !errors.Is(err, expectedError) {
		t.Errorf("Lookup() error (-got, +want):\n-%v\n+%v", err, expectedError)
	}
}

func TestGetEmpty(t *testing.T) {
	r, _, sc := setupTestCache(t)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{}, ErrCacheMiss)

	gotMetrics := r.GetMetrics("akvorado_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`: "0",
		`hit`:     "0",
		`miss`:    "1",
		`size`:    "0",
		`hosts`:   "0",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestSimpleLookup(t *testing.T) {
	r, _, sc := setupTestCache(t)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"}, nil)
	expectCacheLookup(t, sc, "127.0.0.1", 787, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.2", 676, Interface{}, ErrCacheMiss)

	gotMetrics := r.GetMetrics("akvorado_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`: "0",
		`hit`:     "1",
		`miss`:    "2",
		`size`:    "1",
		`hosts`:   "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpire(t *testing.T) {
	r, clock, sc := setupTestCache(t)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)
	sc.Expire(time.Hour)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"}, nil)
	expectCacheLookup(t, sc, "127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"}, nil)
	expectCacheLookup(t, sc, "127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"}, nil)
	sc.Expire(29 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"}, nil)
	expectCacheLookup(t, sc, "127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"}, nil)
	sc.Expire(19 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.1", 678, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"}, nil)
	sc.Expire(9 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.1", 678, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.2", 678, Interface{}, ErrCacheMiss)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Expire(19 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"}, nil)

	gotMetrics := r.GetMetrics("akvorado_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`: "3",
		`hit`:     "7",
		`miss`:    "6",
		`size`:    "1",
		`hosts`:   "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpireRefresh(t *testing.T) {
	_, clock, sc := setupTestCache(t)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)

	// Refresh first entry
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)

	sc.Expire(29 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"}, nil)
	expectCacheLookup(t, sc, "127.0.0.1", 678, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"}, nil)
}

func TestWouldExpire(t *testing.T) {
	_, clock, sc := setupTestCache(t)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)
	// Refresh
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)

	cases := []struct {
		Minutes  time.Duration
		Expected map[string]map[uint]Interface
	}{
		{9, map[string]map[uint]Interface{
			"127.0.0.1": {
				676: Interface{Name: "Gi0/0/0/1", Description: "Transit"},
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
			"127.0.0.2": {
				678: Interface{Name: "Gi0/0/0/1", Description: "IX"},
			},
		}},
		{19, map[string]map[uint]Interface{
			"127.0.0.1": {
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
			"127.0.0.2": {
				678: Interface{Name: "Gi0/0/0/1", Description: "IX"},
			},
		}},
		{29, map[string]map[uint]Interface{
			"127.0.0.1": {
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
		}},
		{39, map[string]map[uint]Interface{}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d minutes", tc.Minutes), func(t *testing.T) {
			got := sc.WouldExpire(tc.Minutes * time.Minute)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("WouldExpire(%d minutes) (-got, +want):\n%s", tc.Minutes, diff)
			}
		})
	}
}

func TestLoadNotExist(t *testing.T) {
	_, _, sc := setupTestCache(t)
	err := sc.Load("/i/do/not/exist")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sc.Load() error:\n%s", err)
	}
}

func TestSaveLoad(t *testing.T) {
	_, clock, sc := setupTestCache(t)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put("127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})

	target := filepath.Join(t.TempDir(), "cache")
	if err := sc.Save(target); err != nil {
		t.Fatalf("sc.Save() error:\n%s", err)
	}

	_, clock, sc = setupTestCache(t)
	clock.Add(30 * time.Minute)
	if err := sc.Load(target); err != nil {
		t.Fatalf("sc.Load() error:\n%s", err)
	}

	sc.Expire(29 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, Interface{}, ErrCacheMiss)
	expectCacheLookup(t, sc, "127.0.0.1", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"}, nil)
	expectCacheLookup(t, sc, "127.0.0.2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"}, nil)
}

func TestLoadMismatchVersion(t *testing.T) {
	_, _, sc := setupTestCache(t)
	sc.Put("127.0.0.1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	target := filepath.Join(t.TempDir(), "cache")

	cacheCurrentVersionNumber++
	if err := sc.Save(target); err != nil {
		cacheCurrentVersionNumber--
		t.Fatalf("sc.Save() error:\n%s", err)
	}
	cacheCurrentVersionNumber--

	// Try to load it
	_, _, sc = setupTestCache(t)
	if err := sc.Load(target); !errors.Is(err, ErrCacheVersion) {
		t.Fatalf("sc.Load() error:\n%s", err)
	}
}
