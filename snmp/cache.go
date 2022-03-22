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
	cacheCurrentVersionNumber = 8
)

// snmpCache represents the SNMP cache.
type snmpCache struct {
	r         *reporter.Reporter
	cache     map[string]*cachedSampler
	cacheLock sync.RWMutex
	clock     clock.Clock

	metrics struct {
		cacheHit      reporter.Counter
		cacheMiss     reporter.Counter
		cacheExpired  reporter.Counter
		cacheSize     reporter.GaugeFunc
		cacheSamplers reporter.GaugeFunc
	}
}

// cachedSampler represents information about a sampler. It includes
// the mapping from ifIndex to interfaces.
type cachedSampler struct {
	Name       string
	Interfaces map[uint]cachedInterface
}

// Interface contains the information about an interface.
type Interface struct {
	Name        string
	Description string
	Speed       uint
}

// cachedInterface contains the information about a cached interface.
type cachedInterface struct {
	LastUpdated int64
	Interface
}

func newSNMPCache(r *reporter.Reporter, clock clock.Clock) *snmpCache {
	sc := &snmpCache{
		r:     r,
		cache: make(map[string]*cachedSampler),
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
			for _, sampler := range sc.cache {
				result += float64(len(sampler.Interfaces))
			}
			return
		})
	sc.metrics.cacheSamplers = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "cache_samplers",
			Help: "Number of samplers in cache.",
		}, func() float64 {
			sc.cacheLock.RLock()
			defer sc.cacheLock.RUnlock()
			return float64(len(sc.cache))
		})
	return sc
}

// Lookup will perform a lookup of the cache. It returns the sampler
// name as well as the requested interface.
func (sc *snmpCache) Lookup(ip string, ifIndex uint) (string, Interface, error) {
	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()
	sampler, ok := sc.cache[ip]
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return "", Interface{}, ErrCacheMiss
	}
	iface, ok := sampler.Interfaces[ifIndex]
	if !ok {
		sc.metrics.cacheMiss.Inc()
		return "", Interface{}, ErrCacheMiss
	}
	sc.metrics.cacheHit.Inc()
	return sampler.Name, iface.Interface, nil
}

// Put a new entry in the cache.
func (sc *snmpCache) Put(ip string, samplerName string, ifIndex uint, iface Interface) {
	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	ciface := cachedInterface{
		LastUpdated: sc.clock.Now().Unix(),
		Interface:   iface,
	}
	sampler, ok := sc.cache[ip]
	if !ok {
		sampler = &cachedSampler{Interfaces: make(map[uint]cachedInterface)}
		sc.cache[ip] = sampler
	}
	sampler.Name = samplerName
	sampler.Interfaces[ifIndex] = ciface
}

// Expire expire entries older than the provided duration.
func (sc *snmpCache) Expire(older time.Duration) (count uint) {
	threshold := sc.clock.Now().Add(-older).Unix()

	sc.cacheLock.Lock()
	defer sc.cacheLock.Unlock()

	for ip, sampler := range sc.cache {
		for ifindex, iface := range sampler.Interfaces {
			if iface.LastUpdated < threshold {
				delete(sampler.Interfaces, ifindex)
				sc.metrics.cacheExpired.Inc()
				count++
			}
		}
		if len(sampler.Interfaces) == 0 {
			delete(sc.cache, ip)
		}
	}
	return
}

// WouldExpire returns a map of interface entries that would expire.
func (sc *snmpCache) WouldExpire(older time.Duration) map[string]map[uint]Interface {
	threshold := sc.clock.Now().Add(-older).Unix()
	result := make(map[string]map[uint]Interface)

	sc.cacheLock.RLock()
	defer sc.cacheLock.RUnlock()

	for ip, sampler := range sc.cache {
		for ifindex, iface := range sampler.Interfaces {
			if iface.LastUpdated < threshold {
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
	cache := map[string]*cachedSampler{}
	if err := decoder.Decode(&cache); err != nil {
		return err
	}
	sc.cacheLock.Lock()
	sc.cache = cache
	sc.cacheLock.Unlock()
	return nil
}
