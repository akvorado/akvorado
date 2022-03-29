package snmp

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"

	"akvorado/reporter"
)

var (
	// ErrCacheMiss is triggered on lookup cache miss
	ErrCacheMiss = errors.New("SNMP cache miss")
	// ErrCacheVersion is triggered when loading a cache from an incompatible version
	ErrCacheVersion = errors.New("SNMP cache version mismatch")
	// cacheCurrentVersionNumber is the current version of the on-disk cache format
	cacheCurrentVersionNumber = 8
)

// snmpCache represents the SNMP cache.
type snmpCache struct {
	r         *reporter.Reporter
	cache     map[string]*cachedExporter
	cacheLock sync.RWMutex
	clock     clock.Clock

	metrics struct {
		cacheHit       reporter.Counter
		cacheMiss      reporter.Counter
		cacheExpired   reporter.Counter
		cacheSize      reporter.GaugeFunc
		cacheExporters reporter.GaugeFunc
	}
}

// cachedExporter represents information about a exporter. It includes
// the mapping from ifIndex to interfaces.
type cachedExporter struct {
	Name       string
	Interfaces map[uint]*cachedInterface
}

// Interface contains the information about an interface.
type Interface struct {
	Name        string
	Description string
	Speed       uint
}

// cachedInterface contains the information about a cached interface.
type cachedInterface struct {
	LastUpdated  int64
	LastAccessed int64
	Interface
}

func newSNMPCache(r *reporter.Reporter, clock clock.Clock) *snmpCache {
	sc := &snmpCache{
		r:     r,
		cache: make(map[string]*cachedExporter),
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
		}, func() (result float64) {
			sc.cacheLock.RLock()
			defer sc.cacheLock.RUnlock()
			for _, exporter := range sc.cache {
				result += float64(len(exporter.Interfaces))
			}
			return
		})
	sc.metrics.cacheExporters = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "cache_exporters",
			Help: "Number of exporters in cache.",
		}, func() float64 {
			sc.cacheLock.RLock()
			defer sc.cacheLock.RUnlock()
			return float64(len(sc.cache))
		})
	return sc
}

// Lookup will perform a lookup of the cache. It returns the exporter
// name as well as the requested interface.
func (sc *snmpCache) Lookup(ip string, ifIndex uint) (string, Interface, error) {
	return sc.lookup(ip, ifIndex, true)
}

func (sc *snmpCache) lookup(ip string, ifIndex uint, touchAccess bool) (string, Interface, error) {
	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()
	exporter, ok := sc.cache[ip]
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return "", Interface{}, ErrCacheMiss
	}
	iface, ok := exporter.Interfaces[ifIndex]
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return "", Interface{}, ErrCacheMiss
	}
	sc.metrics.cacheHit.Inc()
	if touchAccess {
		atomic.StoreInt64(&iface.LastAccessed, sc.clock.Now().Unix())
	}
	return exporter.Name, iface.Interface, nil
}

// Put a new entry in the cache.
func (sc *snmpCache) Put(ip string, exporterName string, ifIndex uint, iface Interface) {
	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	now := sc.clock.Now().Unix()
	ciface := cachedInterface{
		LastUpdated:  now,
		LastAccessed: now,
		Interface:    iface,
	}
	exporter, ok := sc.cache[ip]
	if !ok {
		exporter = &cachedExporter{Interfaces: make(map[uint]*cachedInterface)}
		sc.cache[ip] = exporter
	}
	exporter.Name = exporterName
	exporter.Interfaces[ifIndex] = &ciface
}

// Expire expire entries older than the provided duration (rely on last access).
func (sc *snmpCache) Expire(older time.Duration) (count uint) {
	threshold := sc.clock.Now().Add(-older).Unix()

	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	for ip, exporter := range sc.cache {
		for ifindex, iface := range exporter.Interfaces {
			if iface.LastAccessed < threshold {
				delete(exporter.Interfaces, ifindex)
				sc.metrics.cacheExpired.Inc()
				count++
			}
		}
		if len(exporter.Interfaces) == 0 {
			delete(sc.cache, ip)
		}
	}
	return
}

// Return entries older than the provided duration. If LastAccessed is
// true, rely on last access, otherwise on last update.
func (sc *snmpCache) entriesOlderThan(older time.Duration, lastAccessed bool) map[string]map[uint]Interface {
	threshold := sc.clock.Now().Add(-older).Unix()
	result := make(map[string]map[uint]Interface)

	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()

	for ip, exporter := range sc.cache {
		for ifindex, iface := range exporter.Interfaces {
			what := &iface.LastAccessed
			if !lastAccessed {
				what = &iface.LastUpdated
			}
			if atomic.LoadInt64(what) < threshold {
				rifaces, ok := result[ip]
				if !ok {
					rifaces = make(map[uint]Interface)
					result[ip] = rifaces
				}
				result[ip][ifindex] = iface.Interface
			}
		}
	}
	return result
}

// Need updates returns a map of interface entries that would need to
// be updated. It relies on last update.
func (sc *snmpCache) NeedUpdates(older time.Duration) map[string]map[uint]Interface {
	return sc.entriesOlderThan(older, false)
}

// Need updates returns a map of interface entries that would have
// expired. It relies on last access.
func (sc *snmpCache) WouldExpire(older time.Duration) map[string]map[uint]Interface {
	return sc.entriesOlderThan(older, true)
}

// Save stores the cache to the provided location.
func (sc *snmpCache) Save(cacheFile string) error {
	tmpFile, err := ioutil.TempFile(
		filepath.Dir(cacheFile),
		fmt.Sprintf("%s-*", filepath.Base(cacheFile)))
	if err != nil {
		return fmt.Errorf("unable to create cache file %q: %w", cacheFile, err)
	}
	defer func() {
		tmpFile.Close()           // ignore errors
		os.Remove(tmpFile.Name()) // ignore errors
	}()

	// Write cache
	encoder := gob.NewEncoder(tmpFile)
	if err := encoder.Encode(sc); err != nil {
		return fmt.Errorf("unable to encode cache: %w", err)
	}

	// Move cache to new location
	if err := os.Rename(tmpFile.Name(), cacheFile); err != nil {
		return fmt.Errorf("unable to write cache file %q: %w", cacheFile, err)
	}
	return nil
}

// Load loads the cache from the provided location.
func (sc *snmpCache) Load(cacheFile string) error {
	f, err := os.Open(cacheFile)
	if err != nil {
		return fmt.Errorf("unable to load cache %q: %w", cacheFile, err)
	}
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(sc); err != nil {
		return fmt.Errorf("unable to decode cache: %w", err)
	}
	return nil
}

// GobEncode encodes the SNMP cache.
func (sc *snmpCache) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(&cacheCurrentVersionNumber); err != nil {
		return nil, err
	}
	sc.cacheLock.Lock() // needed because we won't do atomic access
	defer sc.cacheLock.Unlock()
	if err := encoder.Encode(sc.cache); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode decodes the SNMP cache.
func (sc *snmpCache) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	version := cacheCurrentVersionNumber
	if err := decoder.Decode(&version); err != nil {
		return err
	}
	if version != cacheCurrentVersionNumber {
		return ErrCacheVersion
	}
	cache := map[string]*cachedExporter{}
	if err := decoder.Decode(&cache); err != nil {
		return err
	}
	sc.cacheLock.Lock()
	sc.cache = cache
	sc.cacheLock.Unlock()
	return nil
}
