// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package dns provides optional reverse DNS enrichment for outlet flows.
package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	mdns "github.com/miekg/dns"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

type cacheEntry struct {
	name     string
	expires  time.Time
	lastUsed time.Time
}

type pendingEntry struct {
	done chan struct{}
}

// Dependencies define the dependencies of the DNS component.
type Dependencies struct {
	Daemon daemon.Component
}

// Component represents the reverse DNS enrichment component.
type Component struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config Configuration

	metrics metrics

	queue chan netip.Addr

	cacheMu sync.Mutex
	cache   map[netip.Addr]cacheEntry

	pendingMu sync.Mutex
	pending   map[netip.Addr]*pendingEntry

	lookup func(netip.Addr) (string, time.Duration, bool, error)
}

// New creates a new DNS component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	configuration.IncludeSubnets = normalizePrefixes(configuration.IncludeSubnets)
	configuration.ExcludeSubnets = normalizePrefixes(configuration.ExcludeSubnets)
	configuration.TrimSuffixes = normalizeSuffixes(configuration.TrimSuffixes)
	if configuration.Enabled {
		if len(configuration.Resolvers) == 0 {
			return nil, errors.New("DNS enrichment is enabled but no resolver is configured")
		}
		for _, resolver := range configuration.Resolvers {
			if _, _, err := net.SplitHostPort(resolver); err != nil {
				return nil, fmt.Errorf("invalid DNS resolver %q: %w", resolver, err)
			}
		}
	}

	c := Component{
		r:       r,
		config:  configuration,
		queue:   make(chan netip.Addr, configuration.MaxConcurrentQueries*4),
		cache:   map[netip.Addr]cacheEntry{},
		pending: map[netip.Addr]*pendingEntry{},
	}
	c.lookup = c.lookupPTR
	dependencies.Daemon.Track(&c.t, "outlet/dns")
	c.initMetrics()
	return &c, nil
}

// Start starts the DNS component.
func (c *Component) Start() error {
	if !c.config.Enabled {
		return nil
	}
	c.r.Info().Msg("starting DNS enrichment component")
	for range c.config.MaxConcurrentQueries {
		c.t.Go(c.worker)
	}
	return nil
}

// Stop stops the DNS component.
func (c *Component) Stop() error {
	if !c.config.Enabled {
		return nil
	}
	c.r.Info().Msg("stopping DNS enrichment component")
	c.t.Kill(nil)
	err := c.t.Wait()
	c.r.Info().Msg("DNS enrichment component stopped")
	return err
}

// Lookup returns a hostname for an IP address on cache hit. On cache miss, it
// enqueues an asynchronous PTR lookup. It returns an empty string immediately by
// default, or waits briefly for the first result when configured to do so.
func (c *Component) Lookup(ip netip.Addr) string {
	if c == nil || !c.config.Enabled || !ip.IsValid() || ip.IsUnspecified() {
		return ""
	}
	ip = normalizeAddr(ip)
	if !c.shouldResolve(ip) {
		return ""
	}

	now := time.Now()
	if name, ok := c.getCached(now, ip); ok {
		c.metrics.cacheHits.Inc()
		return name
	}
	c.metrics.cacheMisses.Inc()

	entry, shouldQueue := c.getOrCreatePending(ip)
	if shouldQueue {
		select {
		case c.queue <- ip:
		default:
			c.clearPending(ip)
			c.metrics.errors.WithLabelValues("queue full").Inc()
			return ""
		}
	}
	if !c.config.WaitForInitialResult {
		return ""
	}
	return c.waitForInitialResult(ip, entry)
}

func (c *Component) waitForInitialResult(ip netip.Addr, entry *pendingEntry) string {
	timer := time.NewTimer(c.config.InitialTimeout)
	defer timer.Stop()
	select {
	case <-entry.done:
	case <-timer.C:
		return ""
	}
	if name, ok := c.getCached(time.Now(), ip); ok {
		return name
	}
	return ""
}

func (c *Component) getCached(now time.Time, ip netip.Addr) (string, bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	entry, ok := c.cache[ip]
	if !ok {
		return "", false
	}
	if !now.Before(entry.expires) {
		delete(c.cache, ip)
		return "", false
	}
	entry.lastUsed = now
	c.cache[ip] = entry
	return entry.name, true
}

func (c *Component) putCached(ip netip.Addr, name string, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	now := time.Now()
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if len(c.cache) >= c.config.Cache.MaxEntries {
		c.evictLRU()
	}
	c.cache[ip] = cacheEntry{
		name:     name,
		expires:  now.Add(ttl),
		lastUsed: now,
	}
}

func (c *Component) evictLRU() {
	var (
		oldestIP   netip.Addr
		oldestTime time.Time
	)
	for ip, entry := range c.cache {
		if oldestTime.IsZero() || entry.lastUsed.Before(oldestTime) {
			oldestIP = ip
			oldestTime = entry.lastUsed
		}
	}
	if oldestIP.IsValid() {
		delete(c.cache, oldestIP)
	}
}

func (c *Component) getOrCreatePending(ip netip.Addr) (*pendingEntry, bool) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if entry, ok := c.pending[ip]; ok {
		return entry, false
	}
	entry := &pendingEntry{done: make(chan struct{})}
	c.pending[ip] = entry
	return entry, true
}

func (c *Component) clearPending(ip netip.Addr) {
	c.pendingMu.Lock()
	entry, ok := c.pending[ip]
	if ok {
		delete(c.pending, ip)
		close(entry.done)
	}
	c.pendingMu.Unlock()
}

func (c *Component) worker() error {
	for {
		select {
		case <-c.t.Dying():
			return nil
		case ip := <-c.queue:
			c.resolve(ip)
			c.clearPending(ip)
		}
	}
}

func (c *Component) resolve(ip netip.Addr) {
	start := time.Now()
	name, ttl, negative, err := c.lookup(ip)
	c.metrics.queryDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		c.metrics.errors.WithLabelValues(err.Error()).Inc()
		return
	}
	if negative {
		c.putCached(ip, "", c.config.Cache.NegativeTTL)
		return
	}
	name = c.normalizeName(name)
	if name == "" {
		c.putCached(ip, "", c.config.Cache.NegativeTTL)
		return
	}
	c.putCached(ip, name, c.clampTTL(ttl))
}

func (c *Component) lookupPTR(ip netip.Addr) (string, time.Duration, bool, error) {
	ptrName, err := mdns.ReverseAddr(ip.Unmap().String())
	if err != nil {
		return "", 0, false, errors.New("reverse addr")
	}

	var lastErr error
	for _, resolver := range c.config.Resolvers {
		for range c.config.Attempts {
			c.metrics.queries.Inc()
			name, ttl, negative, err := c.exchange(resolver, ptrName, "udp")
			if err == nil {
				return name, ttl, negative, nil
			}
			if errors.Is(err, errTruncated) {
				c.metrics.queries.Inc()
				name, ttl, negative, err = c.exchange(resolver, ptrName, "tcp")
				if err == nil {
					return name, ttl, negative, nil
				}
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("query failed")
	}
	return "", 0, false, lastErr
}

var errTruncated = errors.New("truncated")

func (c *Component) exchange(resolver, ptrName, network string) (string, time.Duration, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	query := new(mdns.Msg)
	query.SetQuestion(ptrName, mdns.TypePTR)
	client := mdns.Client{Net: network, Timeout: c.config.Timeout}
	response, _, err := client.ExchangeContext(ctx, query, resolver)
	if err != nil {
		return "", 0, false, errors.New("query")
	}
	if response == nil {
		return "", 0, false, errors.New("empty response")
	}
	if response.Truncated && network == "udp" {
		return "", 0, false, errTruncated
	}
	if response.Rcode == mdns.RcodeNameError {
		return "", 0, true, nil
	}
	if response.Rcode != mdns.RcodeSuccess {
		return "", 0, false, fmt.Errorf("rcode %s", mdns.RcodeToString[response.Rcode])
	}

	for _, answer := range response.Answer {
		if ptr, ok := answer.(*mdns.PTR); ok {
			return ptr.Ptr, time.Duration(ptr.Hdr.Ttl) * time.Second, false, nil
		}
	}
	return "", 0, true, nil
}

func (c *Component) shouldResolve(ip netip.Addr) bool {
	for _, prefix := range c.config.ExcludeSubnets {
		if prefix.Contains(ip) {
			return false
		}
	}
	if len(c.config.IncludeSubnets) == 0 {
		return true
	}
	for _, prefix := range c.config.IncludeSubnets {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func (c *Component) clampTTL(ttl time.Duration) time.Duration {
	if ttl < c.config.Cache.MinTTL {
		return c.config.Cache.MinTTL
	}
	if ttl > c.config.Cache.MaxTTL {
		return c.config.Cache.MaxTTL
	}
	return ttl
}

func (c *Component) normalizeName(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	lower := strings.ToLower(name)
	for _, suffix := range c.config.TrimSuffixes {
		if strings.HasSuffix(lower, suffix) {
			name = name[:len(name)-len(suffix)]
			lower = lower[:len(lower)-len(suffix)]
			name = strings.TrimSuffix(name, ".")
			lower = strings.TrimSuffix(lower, ".")
		}
	}
	return name
}

func normalizeAddr(ip netip.Addr) netip.Addr {
	return helpers.AddrTo6(ip.Unmap())
}

func normalizePrefixes(prefixes []netip.Prefix) []netip.Prefix {
	normalized := make([]netip.Prefix, 0, len(prefixes))
	for _, prefix := range prefixes {
		if !prefix.IsValid() {
			continue
		}
		normalized = append(normalized, helpers.PrefixTo6(prefix.Masked()))
	}
	return normalized
}

func normalizeSuffixes(suffixes []string) []string {
	normalized := make([]string, 0, len(suffixes))
	for _, suffix := range suffixes {
		suffix = strings.TrimSpace(strings.ToLower(suffix))
		suffix = strings.TrimSuffix(suffix, ".")
		if suffix == "" {
			continue
		}
		normalized = append(normalized, suffix)
	}
	return normalized
}
