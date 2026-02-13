// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"encoding/base64"
	"net"
	"os"
	"sync"

	lru "github.com/hashicorp/golang-lru"
)

// Anonymizer wraps CryptoPAN and an LRU cache.
type Anonymizer struct {
	cp    *CryptoPAN
	cache *lru.Cache
	mu    sync.RWMutex

	enabled   bool
	aggregate bool

	aggregateV4Len int
	aggregateV6Len int
}

// NewAnonymizer builds an Anonymizer from the new nested configuration.
// - Mode == "cryptopan": uses CryptoPAN with provided key/cache.
// - Mode == "aggregate": no CryptoPAN, only aggregation using provided prefixes.
// If cfg.Enabled is false the returned Anonymizer will be disabled.
func NewAnonymizer(cfg AnonymizeConfig) (*Anonymizer, error) {
	// prepare cache size (use crypto cache if present, else default)
	cacheSize := cfg.CryptoPan.Cache
	if cacheSize <= 0 {
		cacheSize = DefaultConfiguration().Anonymize.CryptoPan.Cache
	}
	c, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}

	a := &Anonymizer{
		cache:          c,
		enabled:        cfg.Enabled,
		aggregate:      false,
		aggregateV4Len: cfg.Aggregate.V4Prefix,
		aggregateV6Len: cfg.Aggregate.V6Prefix,
	}

	// If not enabled, return quickly (cache allocated for safety)
	if !cfg.Enabled {
		return a, nil
	}

	switch cfg.Mode {
	case AnonymizeModeAggregate:
		// aggregate-only: no CryptoPAN needed
		a.aggregate = true
		a.cp = nil
	default:
		// cryptopan mode (default)
		keyStr := cfg.CryptoPan.Key
		if keyStr == "" {
			// fallback to environment var
			keyStr = os.Getenv("CRYPTOPAN_KEY")
		}
		if keyStr == "" {
			// no key -> disable anonymizer (but aggregation might still be desired; keep enabled=false)
			return &Anonymizer{cache: c, enabled: false}, nil
		}
		// Try base64 decode; if fails, use raw bytes
		key, err := base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			key = []byte(keyStr)
		}
		cp, err := NewCryptoPAN(key)
		if err != nil {
			return nil, err
		}
		a.cp = cp
		a.aggregate = false
	}

	// Ensure sane defaults for prefixes
	if a.aggregateV4Len == 0 {
		a.aggregateV4Len = DefaultConfiguration().Anonymize.Aggregate.V4Prefix
	}
	if a.aggregateV6Len == 0 {
		a.aggregateV6Len = DefaultConfiguration().Anonymize.Aggregate.V6Prefix
	}

	return a, nil
}

// AnonymizeIP returns an anonymized copy of ip. Non-IPv4/IPv6 addresses return original ip.
func (a *Anonymizer) AnonymizeIP(ip net.IP) net.IP {
	if !a.enabled || ip == nil {
		return ip
	}
	key := ip.String()

	// cache read
	a.mu.RLock()
	if v, ok := a.cache.Get(key); ok {
		a.mu.RUnlock()
		if cached, ok2 := v.(net.IP); ok2 {
			return append(net.IP(nil), cached...)
		}
	} else {
		a.mu.RUnlock()
	}

	// If CryptoPAN is not configured, return original ip
	if a.cp == nil {
		return ip
	}

	var anon net.IP
	if ip.To4() != nil {
		anon = a.cp.AnonymizeIPv4(ip)
	} else {
		anon = a.cp.AnonymizeIPv6(ip)
	}

	// cache write
	a.mu.Lock()
	a.cache.Add(key, append(net.IP(nil), anon...))
	a.mu.Unlock()
	return anon
}

// aggregateIP helper aggregates the given IP to the specified prefix length and returns a copy.
func aggregateIP(ip net.IP, prefix int) net.IP {
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		mask := net.CIDRMask(prefix, 32)
		res := v4.Mask(mask)
		return append(net.IP(nil), res...)
	}
	// IPv6
	ip6 := ip.To16()
	if ip6 == nil {
		return nil
	}
	mask := net.CIDRMask(prefix, 128)
	res := ip6.Mask(mask)
	return append(net.IP(nil), res...)
}

// AggregateIP returns an aggregated copy of ip. Non-IPv4/IPv6 addresses return original ip.
func (a *Anonymizer) AggregateIP(ip net.IP) net.IP {
	if !a.enabled || ip == nil {
		return ip
	}

	key := ip.String()
	// cache read
	a.mu.RLock()
	if v, ok := a.cache.Get(key); ok {
		a.mu.RUnlock()
		if cached, ok2 := v.(net.IP); ok2 {
			return append(net.IP(nil), cached...)
		}
	} else {
		a.mu.RUnlock()
	}

	// Only aggregate the original IP, do not anonymize it.
	var agg net.IP
	if ip.To4() != nil {
		if a.aggregate {
			agg = aggregateIP(ip, a.aggregateV4Len) // e.g. /24
		} else {
			agg = append(net.IP(nil), ip.To4()...)
		}
	} else {
		if a.aggregate {
			agg = aggregateIP(ip, a.aggregateV6Len) // e.g. /64
		} else {
			agg = append(net.IP(nil), ip.To16()...)
		}
	}

	// cache write
	a.mu.Lock()
	a.cache.Add(key, append(net.IP(nil), agg...))
	a.mu.Unlock()
	return agg
}

// AnonymizeFlowFields takes textual one addresses and returns anonymized textual value.
func (a *Anonymizer) AnonymizeFlowFields(addr string) string {
	if !a.enabled {
		return addr
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return addr
	}
	var ai net.IP
	// If we are in aggregate mode, aggregate. Otherwise anonymize via CryptoPAN.
	if a.aggregate {
		ai = a.AggregateIP(ip)
	} else {
		ai = a.AnonymizeIP(ip)
	}
	if ai != nil {
		return ai.String()
	}
	return addr
}
