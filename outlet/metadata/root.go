// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package metadata handles metadata polling to get interface names and
// descriptions. It keeps a cache of retrieved entries and refresh them. It is
// modular and accepts several kind of providers (including SNMP).
package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/eapache/go-resiliency/breaker"
	"golang.org/x/sync/singleflight"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/outlet/metadata/provider"
)

// Component represents the metadata compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	sc *metadataCache
	sf singleflight.Group

	providerBreakersLock   sync.Mutex
	providerBreakerLoggers map[netip.Addr]reporter.Logger
	providerBreakers       map[netip.Addr]*breaker.Breaker
	providers              []provider.Provider
	initialDeadline        time.Time

	metrics struct {
		cacheRefreshRuns         reporter.Counter
		cacheRefresh             reporter.Counter
		providerBreakerOpenCount *reporter.CounterVec
		providerRequests         reporter.Counter
		providerErrors           reporter.Counter
	}
}

// Dependencies define the dependencies of the metadata component.
type Dependencies struct {
	Daemon daemon.Component
}

// ErrQueryTimeout is the error returned when a query timeout.
var ErrQueryTimeout = errors.New("provider query timeout")

// New creates a new metadata component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	if configuration.CacheRefresh > 0 && configuration.CacheRefresh < configuration.CacheDuration {
		return nil, errors.New("cache refresh must be greater than cache duration")
	}
	if configuration.CacheDuration < configuration.CacheCheckInterval {
		return nil, errors.New("cache duration must be greater than cache check interval")
	}

	sc := newMetadataCache(r)
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,
		sc:     sc,

		providerBreakers:       make(map[netip.Addr]*breaker.Breaker),
		providerBreakerLoggers: make(map[netip.Addr]reporter.Logger),
		providers:              make([]provider.Provider, 0, 1),
	}
	c.d.Daemon.Track(&c.t, "outlet/metadata")

	// Initialize providers
	for _, p := range c.config.Providers {
		selectedProvider, err := p.Config.New(r)
		if err != nil {
			return nil, err
		}
		c.providers = append(c.providers, selectedProvider)
	}

	c.metrics.cacheRefreshRuns = r.Counter(
		reporter.CounterOpts{
			Name: "cache_refresh_runs_total",
			Help: "Number of times the cache refresh was triggered.",
		})
	c.metrics.cacheRefresh = r.Counter(
		reporter.CounterOpts{
			Name: "cache_refreshes_total",
			Help: "Number of entries refreshed in cache.",
		})
	c.metrics.providerBreakerOpenCount = r.CounterVec(
		reporter.CounterOpts{
			Name: "provider_breaker_opens_total",
			Help: "Provider breaker was opened due to too many errors.",
		},
		[]string{"exporter"})
	c.metrics.providerRequests = r.Counter(
		reporter.CounterOpts{
			Name: "provider_requests_total",
			Help: "Number of provider requests.",
		})
	c.metrics.providerErrors = r.Counter(
		reporter.CounterOpts{
			Name: "provider_errors_total",
			Help: "Number of provider errors.",
		})
	return &c, nil
}

// Start starts the metadata component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting metadata component")
	c.initialDeadline = time.Now().Add(c.config.InitialDelay)

	// Load cache
	if c.config.CachePersistFile != "" {
		if err := c.sc.Load(c.config.CachePersistFile); err != nil {
			c.r.Err(err).Msg("cannot load cache, ignoring")
		}
	}

	// Goroutine to refresh the cache
	healthyTicker := make(chan reporter.ChannelHealthcheckFunc)
	c.r.RegisterHealthcheck("metadata/ticker", reporter.ChannelHealthcheck(c.t.Context(nil), healthyTicker))
	c.t.Go(func() error {
		c.r.Debug().Msg("starting metadata ticker")
		ticker := time.NewTicker(c.config.CacheCheckInterval)
		defer ticker.Stop()
		defer close(healthyTicker)
		for {
			select {
			case <-c.t.Dying():
				c.r.Debug().Msg("shutting down metadata ticker")
				return nil
			case cb, ok := <-healthyTicker:
				if ok {
					cb(reporter.HealthcheckOK, "ok")
				}
			case <-ticker.C:
				c.expireCache()
			}
		}
	})

	return nil
}

// Stop stops the metadata component
func (c *Component) Stop() error {
	defer func() {
		if c.config.CachePersistFile != "" {
			if err := c.sc.Save(c.config.CachePersistFile); err != nil {
				c.r.Err(err).Msg("cannot save cache")
			}
		}
		c.r.Info().Msg("metadata component stopped")
	}()
	c.r.Info().Msg("stopping metadata component")
	c.t.Kill(nil)
	return c.t.Wait()
}

// Lookup for interface information for the provided exporter and ifIndex. If
// the information is not in the cache, it will be polled from the provider. The
// returned result has a field Found to tell if the lookup is successful or not.
func (c *Component) Lookup(t time.Time, exporterIP netip.Addr, ifIndex uint) provider.Answer {
	query := provider.Query{ExporterIP: exporterIP, IfIndex: ifIndex}

	// Check cache first
	if answer, ok := c.sc.Lookup(t, query); ok {
		return answer
	}

	// Use singleflight to prevent duplicate queries
	key := fmt.Sprintf("%s-%d", exporterIP, ifIndex)
	result, err, _ := c.sf.Do(key, func() (any, error) {
		return c.queryProviders(query)
	})

	if err != nil {
		return provider.Answer{}
	}

	return result.(provider.Answer)
}

// queryProviders queries all providers. It returns the answer for the specific
// query and cache it.
func (c *Component) queryProviders(query provider.Query) (provider.Answer, error) {
	c.metrics.providerRequests.Inc()

	// Check if provider breaker is open
	c.providerBreakersLock.Lock()
	providerBreaker, ok := c.providerBreakers[query.ExporterIP]
	if !ok {
		providerBreaker = breaker.New(20, 1, time.Minute)
		c.providerBreakers[query.ExporterIP] = providerBreaker
	}
	c.providerBreakersLock.Unlock()

	var result provider.Answer
	err := providerBreaker.Run(func() error {
		deadline := time.Now().Add(c.config.QueryTimeout)
		if deadline.Before(c.initialDeadline) {
			deadline = c.initialDeadline
		}
		ctx, cancel := context.WithDeadlineCause(
			c.t.Context(nil),
			deadline,
			ErrQueryTimeout)
		defer cancel()

		now := time.Now()
		for _, p := range c.providers {
			answer, err := p.Query(ctx, query)
			if err == provider.ErrSkipProvider {
				// Next provider
				continue
			}
			if err != nil {
				return err
			}
			c.sc.Put(now, query, answer)
			result = answer
			return nil
		}
		return nil
	})
	if err != nil {
		c.metrics.providerErrors.Inc()
		if err == breaker.ErrBreakerOpen {
			c.metrics.providerBreakerOpenCount.WithLabelValues(query.ExporterIP.Unmap().String()).Inc()
			c.providerBreakersLock.Lock()
			l, ok := c.providerBreakerLoggers[query.ExporterIP]
			if !ok {
				l = c.r.Sample(reporter.BurstSampler(time.Minute, 1)).
					With().
					Str("exporter", query.ExporterIP.Unmap().String()).
					Logger()
				c.providerBreakerLoggers[query.ExporterIP] = l
			}
			l.Warn().Msg("provider breaker open")
			c.providerBreakersLock.Unlock()
		}
		return provider.Answer{}, err
	}

	return result, nil
}

// refreshCacheEntry refreshes a single cache entry.
func (c *Component) refreshCacheEntry(exporterIP netip.Addr, ifIndex uint) {
	query := provider.Query{
		ExporterIP: exporterIP,
		IfIndex:    ifIndex,
	}
	c.queryProviders(query)
}

// expireCache handles cache expiration and refresh.
func (c *Component) expireCache() {
	c.sc.Expire(time.Now().Add(-c.config.CacheDuration))
	if c.config.CacheRefresh > 0 {
		c.r.Debug().Msg("refresh metadata cache")
		c.metrics.cacheRefreshRuns.Inc()
		count := 0
		toRefresh := c.sc.NeedUpdates(time.Now().Add(-c.config.CacheRefresh))
		for exporter, ifaces := range toRefresh {
			for _, ifIndex := range ifaces {
				go c.refreshCacheEntry(exporter, ifIndex)
				count++
			}
		}
		c.r.Debug().Int("count", count).Msg("refreshed metadata cache")
		c.metrics.cacheRefresh.Add(float64(count))
	}
}
