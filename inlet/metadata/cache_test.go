// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

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
	"akvorado/inlet/metadata/provider"
)

func setupTestCache(t *testing.T) (*reporter.Reporter, *metadataCache) {
	t.Helper()
	r := reporter.NewMock(t)
	sc := newMetadataCache(r)
	return r, sc
}

func expectCacheLookup(t *testing.T, sc *metadataCache, exporterIP string, ifIndex uint, expected provider.Answer) {
	t.Helper()
	ip := netip.MustParseAddr(exporterIP)
	ip = netip.AddrFrom16(ip.As16())
	got, ok := sc.Lookup(time.Time{}, provider.Query{
		ExporterIP: ip,
		IfIndex:    ifIndex,
	})
	if ok && (got == provider.Answer{}) {
		t.Error("Lookup() returned an empty result")
	} else if !ok && (got != provider.Answer{}) {
		t.Error("Lookup() returned a non-empty result")
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Lookup() (-got, +want):\n%s", diff)
	}
}

func TestGetEmpty(t *testing.T) {
	r, sc := setupTestCache(t)
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{})

	gotMetrics := r.GetMetrics("akvorado_inlet_metadata_cache_")
	expectedMetrics := map[string]string{
		`expired_entries_total`: "0",
		`hits_total`:            "0",
		`misses_total`:          "1",
		`size_entries`:          "0",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestSimpleLookup(t *testing.T) {
	r, sc := setupTestCache(t)
	sc.Put(time.Now(),
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit", Speed: 1000},
		})
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit", Speed: 1000},
	})
	expectCacheLookup(t, sc, "127.0.0.1", 787, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.2", 676, provider.Answer{})

	gotMetrics := r.GetMetrics("akvorado_inlet_metadata_cache_")
	expectedMetrics := map[string]string{
		`expired_entries_total`: "0",
		`hits_total`:            "1",
		`misses_total`:          "2",
		`size_entries`:          "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpire(t *testing.T) {
	r, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost2",
			Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.2"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost3",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
		})
	now = now.Add(10 * time.Minute)
	sc.Expire(now.Add(-time.Hour))
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
	})
	expectCacheLookup(t, sc, "127.0.0.1", 678, provider.Answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
	})
	expectCacheLookup(t, sc, "127.0.0.2", 678, provider.Answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
	})
	sc.Expire(now.Add(-29 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.1", 678, provider.Answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
	})
	expectCacheLookup(t, sc, "127.0.0.2", 678, provider.Answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
	})
	sc.Expire(now.Add(-19 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.1", 678, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.2", 678, provider.Answer{
		ExporterName: "localhost3",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
	})
	sc.Expire(now.Add(-9 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.1", 678, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.2", 678, provider.Answer{})
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
		})
	now = now.Add(10 * time.Minute)
	sc.Expire(now.Add(-19 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
	})

	gotMetrics := r.GetMetrics("akvorado_inlet_metadata_cache_")
	expectedMetrics := map[string]string{
		`expired_entries_total`: "3",
		`hits_total`:            "7",
		`misses_total`:          "6",
		`size_entries`:          "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

func TestExpireRefresh(t *testing.T) {
	_, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.2"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost2",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
		})
	now = now.Add(10 * time.Minute)

	// Refresh first entry
	sc.Lookup(now, provider.Query{
		ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
		IfIndex:    676,
	})
	now = now.Add(10 * time.Minute)

	sc.Expire(now.Add(-29 * time.Minute))
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
	})
	expectCacheLookup(t, sc, "127.0.0.1", 678, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.2", 678, provider.Answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
	})
}

func TestNeedUpdates(t *testing.T) {
	_, sc := setupTestCache(t)
	now := time.Now()
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.2"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost2",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX"},
		})
	now = now.Add(10 * time.Minute)
	// Refresh
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost1",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
		})
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
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    676,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.1"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost",
			Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
		})
	now = now.Add(10 * time.Minute)
	sc.Put(now,
		provider.Query{
			ExporterIP: netip.MustParseAddr("::ffff:127.0.0.2"),
			IfIndex:    678,
		},
		provider.Answer{
			ExporterName: "localhost2",
			Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX", Speed: 1000},
		})

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
	expectCacheLookup(t, sc, "127.0.0.1", 676, provider.Answer{})
	expectCacheLookup(t, sc, "127.0.0.1", 678, provider.Answer{
		ExporterName: "localhost",
		Interface:    Interface{Name: "Gi0/0/0/2", Description: "Peering"},
	})
	expectCacheLookup(t, sc, "127.0.0.2", 678, provider.Answer{
		ExporterName: "localhost2",
		Interface:    Interface{Name: "Gi0/0/0/1", Description: "IX", Speed: 1000},
	})
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
				sc.Put(now, provider.Query{
					ExporterIP: netip.MustParseAddr(fmt.Sprintf("::ffff:127.0.0.%d", ip)),
					IfIndex:    uint(iface),
				}, provider.Answer{
					ExporterName: fmt.Sprintf("localhost%d", ip),
					Interface:    Interface{Name: "Gi0/0/0/1", Description: "Transit"},
				})
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
				sc.Lookup(now, provider.Query{
					ExporterIP: netip.MustParseAddr(fmt.Sprintf("::ffff:127.0.0.%d", ip)),
					IfIndex:    uint(iface),
				})
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

	gotMetrics := r.GetMetrics("akvorado_inlet_metadata_cache_")
	hits, _ := strconv.Atoi(gotMetrics["hits_total"])
	misses, _ := strconv.Atoi(gotMetrics["misses_total"])
	if int64(hits+misses) != atomic.LoadInt64(&lookups) {
		t.Errorf("hit + miss = %d, expected %d", hits+misses, atomic.LoadInt64(&lookups))
	}
}
