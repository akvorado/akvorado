// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import (
	"net/netip"
	"time"

	"akvorado/common/helpers/cache"
	"akvorado/common/reporter"
	"akvorado/outlet/metadata/provider"
)

// Interface describes an interface.
type Interface = provider.Interface

// metadataCache represents the metadata cache.
type metadataCache struct {
	r     *reporter.Reporter
	cache *cache.Cache[provider.Query, provider.Answer]

	metrics struct {
		cacheHit     reporter.Counter
		cacheMiss    reporter.Counter
		cacheExpired reporter.Counter
		cacheSize    reporter.GaugeFunc
	}
}

func newMetadataCache(r *reporter.Reporter) *metadataCache {
	sc := &metadataCache{
		r:     r,
		cache: cache.New[provider.Query, provider.Answer](),
	}
	sc.metrics.cacheHit = r.Counter(
		reporter.CounterOpts{
			Name: "cache_hits_total",
			Help: "Number of lookups retrieved from cache.",
		})
	sc.metrics.cacheMiss = r.Counter(
		reporter.CounterOpts{
			Name: "cache_misses_total",
			Help: "Number of lookup miss.",
		})
	sc.metrics.cacheExpired = r.Counter(
		reporter.CounterOpts{
			Name: "cache_expired_entries_total",
			Help: "Number of cache entries expired.",
		})
	sc.metrics.cacheSize = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "cache_size_entries",
			Help: "Number of entries in cache.",
		}, func() float64 {
			return float64(sc.cache.Size())
		})
	return sc
}

// Lookup will perform a lookup of the cache. It returns the exporter
// name as well as the requested interface.
func (sc *metadataCache) Lookup(t time.Time, query provider.Query) (provider.Answer, bool) {
	result, ok := sc.cache.Get(t, query)
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return provider.Answer{}, false
	}
	sc.metrics.cacheHit.Inc()
	return result, true
}

// Put a new entry in the cache.
func (sc *metadataCache) Put(t time.Time, query provider.Query, answer provider.Answer) {
	sc.cache.Put(t, query, answer)
}

// Expire expire entries whose last access is before the provided time
func (sc *metadataCache) Expire(before time.Time) int {
	expired := sc.cache.DeleteLastAccessedBefore(before)
	sc.metrics.cacheExpired.Add(float64(expired))
	return expired
}

// NeedUpdates returns a map of interface entries that would need to
// be updated. It relies on last update.
func (sc *metadataCache) NeedUpdates(before time.Time) map[netip.Addr][]uint {
	result := map[netip.Addr][]uint{}
	for k := range sc.cache.ItemsLastUpdatedBefore(before) {
		interfaces, ok := result[k.ExporterIP]
		if !ok {
			interfaces = []uint{}
		}
		result[k.ExporterIP] = append(interfaces, k.IfIndex)
	}
	return result
}

// Save stores the cache to the provided location.
func (sc *metadataCache) Save(cacheFile string) error {
	return sc.cache.Save(cacheFile)
}

// Load loads the cache from the provided location.
func (sc *metadataCache) Load(cacheFile string) error {
	return sc.cache.Load(cacheFile)
}
