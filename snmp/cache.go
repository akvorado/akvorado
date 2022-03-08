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
	cacheCurrentVersionNumber = 5
)

// snmpCache represents the SNMP cache.
type snmpCache struct {
	r         *reporter.Reporter
	cache     map[string]cachedInterfaces
	cacheLock sync.RWMutex
	clock     clock.Clock

	metrics struct {
		cacheHit     reporter.Counter
		cacheMiss    reporter.Counter
		cacheExpired reporter.Counter
		cacheSize    reporter.GaugeFunc
		cacheHosts   reporter.GaugeFunc
	}
}

// cachedInterfaces represents a mapping from ifIndex to a cached interface.
type cachedInterfaces map[uint]Interface

// Interface contains the information about an interface.
type Interface struct {
	lastUpdated time.Time
	Name        string
	Description string
}

func newSNMPCache(r *reporter.Reporter, clock clock.Clock) *snmpCache {
	sc := &snmpCache{
		r:     r,
		cache: make(map[string]cachedInterfaces),
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
			for _, ifaces := range sc.cache {
				result += float64(len(ifaces))
			}
			return
		})
	sc.metrics.cacheHosts = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "cache_hosts",
			Help: "Number of hosts in cache.",
		}, func() float64 {
			sc.cacheLock.RLock()
			defer sc.cacheLock.RUnlock()
			return float64(len(sc.cache))
		})
	return sc
}

// Lookup will perform a lookup of the cache
func (sc *snmpCache) Lookup(host string, ifIndex uint) (Interface, error) {
	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()
	ifaces, ok := sc.cache[host]
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return Interface{}, ErrCacheMiss
	}
	iface, ok := ifaces[ifIndex]
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return Interface{}, ErrCacheMiss
	}
	sc.metrics.cacheHit.Inc()
	return iface, nil
}

// Put a new entry in the cache.
func (sc *snmpCache) Put(host string, ifIndex uint, iface Interface) {
	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()
	ifaces, ok := sc.cache[host]
	if !ok {
		ifaces = cachedInterfaces{}
		sc.cache[host] = ifaces
	}
	iface.lastUpdated = sc.clock.Now()
	ifaces[ifIndex] = iface
}

// Expire expire entries older than the provided duration.
func (sc *snmpCache) Expire(older time.Duration) (count uint) {
	threshold := sc.clock.Now().Add(-older)

	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	for host, ifaces := range sc.cache {
		for ifindex, iface := range ifaces {
			if iface.lastUpdated.Before(threshold) {
				delete(ifaces, ifindex)
				sc.metrics.cacheExpired.Inc()
				count++
			}
		}
		if len(ifaces) == 0 {
			delete(sc.cache, host)
		}
	}
	return
}

// WouldExpire returns a map of interface entries that would expire.
func (sc *snmpCache) WouldExpire(older time.Duration) map[string]map[uint]Interface {
	threshold := sc.clock.Now().Add(-older)
	result := make(map[string]map[uint]Interface)

	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()

	for host, ifaces := range sc.cache {
		for ifindex, iface := range ifaces {
			if iface.lastUpdated.Before(threshold) {
				rifaces, ok := result[host]
				if !ok {
					rifaces = make(map[uint]Interface)
					result[host] = rifaces
				}
				result[host][ifindex] = iface
			}
		}
	}
	return result
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
	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()
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
	cache := map[string]cachedInterfaces{}
	if err := decoder.Decode(&cache); err != nil {
		return err
	}
	sc.cacheLock.Lock()
	sc.cache = cache
	sc.cacheLock.Unlock()
	return nil
}

// GobEncode encodes an interface, including last updated.
func (i Interface) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(i.Name); err != nil {
		return nil, err
	}
	if err := encoder.Encode(i.Description); err != nil {
		return nil, err
	}
	if err := encoder.Encode(i.lastUpdated); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode decodes an interface, including last updated.
func (i *Interface) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	if err := decoder.Decode(&i.Name); err != nil {
		return err
	}
	if err := decoder.Decode(&i.Description); err != nil {
		return err
	}
	if err := decoder.Decode(&i.lastUpdated); err != nil {
		return err
	}
	return nil
}
