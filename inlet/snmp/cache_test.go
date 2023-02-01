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

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func setupTestCache(t *testing.T) (*reporter.Reporter, *snmpCache) {
	t.Helper()
	r := reporter.NewMock(t)
	sc := newSNMPCache(r)
	return r, sc
}

type answer struct {
	ExporterName string
	Interface    Interface
	NOk          bool
}

func expectCacheLookup(t *testing.T, sc *snmpCache, exporterIP string, ifIndex uint, expected answer) {
	t.Helper()
	ip := netip.MustParseAddr(exporterIP)
	ip = netip.AddrFrom16(ip.As16())
	gotExporterName, gotInterface, ok := sc.Lookup(time.Time{}, ip, ifIndex)
	got := answer{gotExporterName, gotInterface, !ok}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
}

func TestGetEmpty(t *testing.T) {
	r, sc := setupTestCache(t)
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{NOk: true})

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`: "0",
		`hit`:     "0",
		`miss`:    "1",
		`size`:    "0",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestSimpleLookup(t *testing.T) {
	r, sc := setupTestCache(t)
	sc.Put(time.Now(), netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit", Speed: 1000})
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit", Speed: 1000}})
	expectCacheLookup(t, sc, "127.0.0.1", 787, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.2", 676, answer{NOk: true})

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`: "0",
		`hit`:     "1",
		`miss`:    "2",
		`size`:    "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpire(t *testing.T) {
	r, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost2", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.2"), "localhost3", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	now = now.Add(10 * time.Minute)
	sc.Expire(now.Add(-time.Hour))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"}})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"}})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
	sc.Expire(now.Add(-29 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"}})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
	sc.Expire(now.Add(-19 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
	sc.Expire(now.Add(-9 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{NOk: true})
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	now = now.Add(10 * time.Minute)
	sc.Expire(now.Add(-19 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"}})

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	expectedMetrics := map[string]string{
		`expired`: "3",
		`hit`:     "7",
		`miss`:    "6",
		`size`:    "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpireRefresh(t *testing.T) {
	_, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	now = now.Add(10 * time.Minute)

	// Refresh first entry
	sc.Lookup(now, netip.MustParseAddr("::ffff:127.0.0.1"), 676)
	now = now.Add(10 * time.Minute)

	sc.Expire(now.Add(-29 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"}})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"}})
}

func TestNeedUpdates(t *testing.T) {
	_, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX"})
	now = now.Add(10 * time.Minute)
	// Refresh
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost1", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	now = now.Add(10 * time.Minute)

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
			got := sc.NeedUpdates(now.Add(-tc.Minutes * time.Minute))
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("WouldExpire(%d minutes) (-got, +want):\n%s", tc.Minutes, diff)
			}
		})
	}
}

func TestLoadNotExist(t *testing.T) {
	_, sc := setupTestCache(t)
	err := sc.Load("/i/do/not/exist")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sc.Load() error:\n%s", err)
	}
}

func TestSaveLoad(t *testing.T) {
	_, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 676, Interface{Name: "Gi0/0/0/1", Description: "Transit"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.1"), "localhost", 678, Interface{Name: "Gi0/0/0/2", Description: "Peering"})
	now = now.Add(10 * time.Minute)
	sc.Put(now, netip.MustParseAddr("::ffff:127.0.0.2"), "localhost2", 678, Interface{Name: "Gi0/0/0/1", Description: "IX", Speed: 1000})

	target := filepath.Join(t.TempDir(), "cache")
	if err := sc.Save(target); err != nil {
		t.Fatalf("sc.Save() error:\n%s", err)
	}

	_, sc = setupTestCache(t)
	now = now.Add(10 * time.Minute)
	if err := sc.Load(target); err != nil {
		t.Fatalf("sc.Load() error:\n%s", err)
	}

	sc.Expire(now.Add(-29 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, answer{NOk: true})
	expectCacheLookup(t, sc, "127.0.0.1", 678, answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"}})
	expectCacheLookup(t, sc, "127.0.0.2", 678, answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX", Speed: 1000}})
}

func TestConcurrentOperations(t *testing.T) {
	r, sc := setupTestCache(t)
	now := time.Now()
	done := make(chan bool)
	var wg sync.WaitGroup
	var nowLock sync.RWMutex

	// Make the clock go forward
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			nowLock.Lock()
			now = now.Add(1 * time.Minute)
			nowLock.Unlock()
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
				nowLock.RLock()
				now := now
				nowLock.RUnlock()
				ip := rand.Intn(10)
				iface := rand.Intn(100)
				sc.Put(now, netip.MustParseAddr(fmt.Sprintf("::ffff:127.0.0.%d", ip)),
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
				nowLock.RLock()
				now := now
				nowLock.RUnlock()
				ip := rand.Intn(10)
				iface := rand.Intn(100)
				sc.Lookup(now, netip.MustParseAddr(fmt.Sprintf("::ffff:127.0.0.%d", ip)),
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
	nowLock.RLock()
	sc.Expire(now.Add(-5 * time.Minute))
	nowLock.RUnlock()
	time.Sleep(30 * time.Millisecond)
	close(done)
	wg.Wait()

	gotMetrics := r.GetMetrics("akvorado_inlet_snmp_cache_")
	hits, _ := strconv.Atoi(gotMetrics["hit"])
	misses, _ := strconv.Atoi(gotMetrics["miss"])
	if int64(hits+misses) != atomic.LoadInt64(&lookups) {
		t.Errorf("hit + miss = %d, expected %d", hits+misses, atomic.LoadInt64(&lookups))
	}
}
