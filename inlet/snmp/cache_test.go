// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"net/netip"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func setupTestCache(t *testing.T) (*reporter.Reporter, *clock.Mock, *snmpCache) {
	t.Helper()
	r := reporter.NewMock(t)
	clock := clock.NewMock()
	sc := newSNMPCache(r, clock)
	return r, clock, sc
}

type answer struct {
	ExporterName string
	Interface    Interface
	Err          error
}

func expectCacheLookup(t *testing.T, sc *snmpCache, exporterIP string, ifIndex uint, expected answer) {
	t.Helper()
	ip := netip.MustParseAddr(exporterIP)
	ip = netip.AddrFrom16(ip.As16())
	gotExporterName, gotInterface, err := sc.lookup(ip, ifIndex, false)
	got := answer{gotExporterName, gotInterface, err}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
}

func TestGetEmpty(t *testing.T) {
	r, _, sc := setupTestCache(t)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{Err: ErrCacheMiss})

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`:   "0",
		`hit`:       "0",
		`miss`:      "1",
		`size`:      "0",
		`exporters`: "0",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestSimpleLookup(t *testing.T) {
	r, _, sc := setupTestCache(t)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit", Speed: 1000})
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit", Speed: 1000}})
	expectCacheLookup(t, sc, "127.0.0.1", 787, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.2", 676, answer{Err: ErrCacheMiss})

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`:   "0",
		`hit`:       "1",
		`miss`:      "2",
		`size`:      "1",
		`exporters`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpire(t *testing.T) {
	r, clock, sc := setupTestCache(t)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost2", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.2"), "localhost3", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)
	sc.Expire(time.Hour)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"}})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"}})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
	sc.Expire(29 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"}})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
	sc.Expire(19 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
	sc.Expire(9 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{Err: ErrCacheMiss})
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Expire(19 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"}})

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`:   "3",
		`hit`:       "7",
		`miss`:      "6",
		`size`:      "1",
		`exporters`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpireRefresh(t *testing.T) {
	_, clock, sc := setupTestCache(t)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)

	// Refresh first entry
	sc.Lookup(netip.MustParseAddr("::ffff:127.0.0.1"), 676)
	clock.Add(10 * time.Minute)

	sc.Expire(29 * time.Minute)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"}})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
}

func TestWouldExpire(t *testing.T) {
	_, clock, sc := setupTestCache(t)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)
	// Refresh
	sc.Lookup(netip.MustParseAddr("::ffff:127.0.0.1"), 676)
	clock.Add(10 * time.Minute)

	cases := []struct {
		Minutes  time.Duration
		Expected map[string]map[uint]Interface
	}{
		{9, map[string]map[uint]Interface{
			"::ffff:127.0.0.1": {
				676: Interface{Name: "Gi0/0/0/1", Description: "Transit"},
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
			"::ffff:127.0.0.2": {
				678: Interface{Name: "Gi0/0/0/1", Description: "IX"},
			},
		}},
		{19, map[string]map[uint]Interface{
			"::ffff:127.0.0.1": {
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
			"::ffff:127.0.0.2": {
				678: Interface{Name: "Gi0/0/0/1", Description: "IX"},
			},
		}},
		{29, map[string]map[uint]Interface{
			"::ffff:127.0.0.1": {
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

func TestNeedUpdates(t *testing.T) {
	_, clock, sc := setupTestCache(t)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	clock.Add(10 * time.Minute)
	// Refresh
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)

	cases := []struct {
		Minutes  time.Duration
		Expected map[string]map[uint]Interface
	}{
		{9, map[string]map[uint]Interface{
			"::ffff:127.0.0.1": {
				676: Interface{Name: "Gi0/0/0/1", Description: "Transit"},
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
			"::ffff:127.0.0.2": {
				678: Interface{Name: "Gi0/0/0/1", Description: "IX"},
			},
		}},
		{19, map[string]map[uint]Interface{
			"::ffff:127.0.0.1": {
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
			"::ffff:127.0.0.2": {
				678: Interface{Name: "Gi0/0/0/1", Description: "IX"},
			},
		}},
		{29, map[string]map[uint]Interface{
			"::ffff:127.0.0.1": {
				678: Interface{Name: "Gi0/0/0/2", Description: "Peering"},
			},
		}},
		{39, map[string]map[uint]Interface{}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d minutes", tc.Minutes), func(t *testing.T) {
			got := sc.NeedUpdates(tc.Minutes * time.Minute)
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
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	clock.Add(10 * time.Minute)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX", Speed: 1000})

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
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{Err: ErrCacheMiss})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"}})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX", Speed: 1000}})
}

func TestLoadMismatchVersion(t *testing.T) {
	_, _, sc := setupTestCache(t)
	sc.Put(netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
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

func TestConcurrentOperations(t *testing.T) {
	r, clock, sc := setupTestCache(t)
	done := make(chan bool)
	var wg sync.WaitGroup

	// Make the clock go forward
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			clock.Add(1 * time.Minute)
			select {
			case <-done:
				return
			case <-time.After(1 * time.Millisecond):
			}
		}
	}()
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				ip := rand.Intn(10)
				iface := rand.Intn(100)
				sc.Put(netip.MustParseAddr(fmt.Sprintf("::ffff:127.0.0.%d", ip)),
					fmt.Sprintf("localhost%d", ip),
					uint(iface), Interface{Name: "Gi0/0/0/1", Description: "Transit"})
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}
	var lookups int64
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				ip := rand.Intn(10)
				iface := rand.Intn(100)
				sc.Lookup(netip.MustParseAddr(fmt.Sprintf("::ffff:127.0.0.%d", ip)),
					uint(iface))
				atomic.AddInt64(&lookups, 1)
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}
	time.Sleep(30 * time.Millisecond)
	sc.Expire(5 * time.Minute)
	time.Sleep(30 * time.Millisecond)
	close(done)
	wg.Wait()

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	hits, _ := strconv.Atoi(gotMetrics["hit"])
	misses, _ := strconv.Atoi(gotMetrics["miss"])
	size, _ := strconv.Atoi(gotMetrics["size"])
	exporters, _ := strconv.Atoi(gotMetrics["exporters"])
	if int64(hits+misses) != atomic.LoadInt64(&lookups) {
		t.Errorf("hit + miss = %d, expected %d", hits+misses, atomic.LoadInt64(&lookups))
	}
}
