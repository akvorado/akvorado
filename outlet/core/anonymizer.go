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
	cp      *CryptoPAN
	cache   *lru.Cache
	enabled bool
	mu      sync.RWMutex
}

// NewAnonymizer constructs an Anonymizer. key may be base64 or raw bytes.
// If key is empty, the anonymizer is disabled.
func NewAnonymizer(keyStr string, cacheSize int) (*Anonymizer, error) {
	if keyStr == "" {
		// fallback to environment var
		keyStr = os.Getenv("CRYPTOPAN_KEY")
	}
	if keyStr == "" {
		return &Anonymizer{enabled: false}, nil
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
	c, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}
	return &Anonymizer{
		cp:      cp,
		cache:   c,
		enabled: true,
	}, nil
}

// AnonymizeIP returns an anonymized copy of ip. Non-IPv4/IPv6 addresses return original ip.
func (a *Anonymizer) AnonymizeIP(ip net.IP) net.IP {
	if !a.enabled || ip == nil {
		return ip
	}
	key := ip.String()
	if v, ok := a.cache.Get(key); ok {
		if cached, ok2 := v.(net.IP); ok2 {
			return append(net.IP(nil), cached...)
		}
	}
	var anon net.IP
	if ip.To4() != nil {
		anon = a.cp.AnonymizeIPv4(ip)
	} else {
		anon = a.cp.AnonymizeIPv6(ip)
	}
	a.cache.Add(key, append(net.IP(nil), anon...))
	return anon
}

// AnonymizeFlowFields takes textual src/dst addresses and returns anonymized textual values.
func (a *Anonymizer) AnonymizeFlowFields(src, dst string) (string, string) {
	if !a.enabled {
		return src, dst
	}
	sip := net.ParseIP(src)
	dip := net.ParseIP(dst)
	as := a.AnonymizeIP(sip)
	ad := a.AnonymizeIP(dip)
	if as == nil {
		as = sip
	}
	if ad == nil {
		ad = dip
	}
	return as.String(), ad.String()
}
