// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"net/netip"
	"time"

	"github.com/benbjohnson/clock"

	"akvorado/common/helpers/cache"
	"akvorado/common/reporter"
)

// snmpCache represents the SNMP cache.
type snmpCache struct {
	r     *reporter.Reporter
	cache *cache.Cache[key, value]
	clock clock.Clock

	metrics struct {
		cacheHit     reporter.Counter
		cacheMiss    reporter.Counter
		cacheExpired reporter.Counter
		cacheSize    reporter.GaugeFunc
	}
}

// Interface contains the information about an interface.
type Interface struct {
	Name        string
	Description string
	Speed       uint
}

type key struct {
	IP    netip.Addr
	Index uint
}
type value struct {
	ExporterName string
	Interface
}

func newSNMPCache(r *reporter.Reporter, clock clock.Clock) *snmpCache {
	sc := &snmpCache{
		r:     r,
		cache: cache.New[key, value](),
		clock: clock,
	}
	sc.metrics.cacheHit = r.Counter(
		reporter.CounterOpts{
			Name: "cache_hit",
			Help: "Number of lookups retrieved from cache.",
		})
	sc.metrics.cacheMiss = r.Counter(
		reporter.CounterOpts{
			Name: "cache_miss",
			Help: "Number of lookup miss.",
		})
	sc.metrics.cacheExpired = r.Counter(
		reporter.CounterOpts{
			Name: "cache_expired",
			Help: "Number of cache entries expired.",
		})
	sc.metrics.cacheSize = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "cache_size",
			Help: "Number of entries in cache.",
		}, func() float64 {
			return float64(sc.cache.Size())
		})
	return sc
}

// Lookup will perform a lookup of the cache. It returns the exporter
// name as well as the requested interface.
func (sc *snmpCache) Lookup(ip netip.Addr, index uint) (string, Interface, bool) {
	return sc.lookup(ip, index, true)
}

func (sc *snmpCache) lookup(ip netip.Addr, index uint, touchAccess bool) (string, Interface, bool) {
	t := time.Time{}
	if touchAccess {
		t = sc.clock.Now()
	}
	result, ok := sc.cache.Get(t, key{ip, index})
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return "", Interface{}, false
	}
	sc.metrics.cacheHit.Inc()
	return result.ExporterName, result.Interface, true
}

// Put a new entry in the cache.
func (sc *snmpCache) Put(ip netip.Addr, exporterName string, index uint, iface Interface) {
	t := sc.clock.Now()
	sc.cache.Put(t, key{ip, index}, value{
		ExporterName: exporterName,
		Interface:    iface,
	})
}

// Expire expire entries older than the provided duration (rely on last access).
func (sc *snmpCache) Expire(older time.Duration) int {
	threshold := sc.clock.Now().Add(-older)
	expired := sc.cache.DeleteLastAccessedBefore(threshold)
	sc.metrics.cacheExpired.Add(float64(expired))
	return expired
}

// NeedUpdates returns a map of interface entries that would need to
// be updated. It relies on last update.
func (sc *snmpCache) NeedUpdates(older time.Duration) map[netip.Addr]map[uint]Interface {
	t := sc.clock.Now().Add(-older)
	result := map[netip.Addr]map[uint]Interface{}
	for k, v := range sc.cache.ItemsLastUpdatedBefore(t) {
		interfaces, ok := result[k.IP]
		if !ok {
			interfaces = map[uint]Interface{}
			result[k.IP] = interfaces
		}
		interfaces[k.Index] = v.Interface
	}
	return result
}

// Save stores the cache to the provided location.
func (sc *snmpCache) Save(cacheFile string) error {
	return sc.cache.Save(cacheFile)
}

// Load loads the cache from the provided location.
func (sc *snmpCache) Load(cacheFile string) error {
	return sc.cache.Load(cacheFile)
}
