// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package metadata handles metadata polling to get interface names and
// descriptions. It keeps a cache of retrieved entries and refresh them. It is
// modular and accepts several kind of providers (including SNMP).
package metadata

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/eapache/go-resiliency/breaker"
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

	healthyWorkers         chan reporter.ChannelHealthcheckFunc
	providerChannel        chan provider.BatchQuery
	dispatcherChannel      chan provider.Query
	dispatcherBChannel     chan (<-chan bool) // block channel for testing
	providerBreakersLock   sync.Mutex
	providerBreakerLoggers map[netip.Addr]reporter.Logger
	providerBreakers       map[netip.Addr]*breaker.Breaker
	providers              []provider.Provider

	metrics struct {
		cacheRefreshRuns         reporter.Counter
		cacheRefresh             reporter.Counter
		providerBusyCount        *reporter.CounterVec
		providerBreakerOpenCount *reporter.CounterVec
		providerBatchedCount     reporter.Counter
	}
}

// Dependencies define the dependencies of the metadata component.
type Dependencies struct {
	Daemon daemon.Component
	Clock  clock.Clock
}

// New creates a new metadata component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	if configuration.CacheRefresh > 0 && configuration.CacheRefresh < configuration.CacheDuration {
		return nil, errors.New("cache refresh must be greater than cache duration")
	}
	if configuration.CacheDuration < configuration.CacheCheckInterval {
		return nil, errors.New("cache duration must be greater than cache check interval")
	}

	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	sc := newMetadataCache(r)
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,
		sc:     sc,

		providerChannel:        make(chan provider.BatchQuery),
		dispatcherChannel:      make(chan provider.Query, 100*configuration.Workers),
		dispatcherBChannel:     make(chan (<-chan bool)),
		providerBreakers:       make(map[netip.Addr]*breaker.Breaker),
		providerBreakerLoggers: make(map[netip.Addr]reporter.Logger),
		providers:              make([]provider.Provider, 0, 1),
	}
	c.d.Daemon.Track(&c.t, "outlet/metadata")

	// Initialize providers
	for _, p := range c.config.Providers {
		selectedProvider, err := p.Config.New(r, func(update provider.Update) {
			c.sc.Put(c.d.Clock.Now(), update.Query, update.Answer)
		})
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
			Name: "cache_refreshs_total",
			Help: "Number of entries refreshed in cache.",
		})
	c.metrics.providerBusyCount = r.CounterVec(
		reporter.CounterOpts{
			Name: "provider_dropped_requests_total",
			Help: "Providers where too busy and dropped requests.",
		},
		[]string{"exporter"})
	c.metrics.providerBreakerOpenCount = r.CounterVec(
		reporter.CounterOpts{
			Name: "provider_breaker_opens_total",
			Help: "Provider breaker was opened due to too many errors.",
		},
		[]string{"exporter"})
	c.metrics.providerBatchedCount = r.Counter(
		reporter.CounterOpts{
			Name: "provider_batched_requests_total",
			Help: "Several requests were batched into one.",
		},
	)
	return &c, nil
}

// Start starts the metadata component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting metadata component")

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
		ticker := c.d.Clock.Ticker(c.config.CacheCheckInterval)
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

	// Goroutine to fetch incoming requests and dispatch them to workers
	healthyDispatcher := make(chan reporter.ChannelHealthcheckFunc)
	c.r.RegisterHealthcheck("metadata/dispatcher", reporter.ChannelHealthcheck(c.t.Context(nil), healthyDispatcher))
	c.t.Go(func() error {
		dying := c.t.Dying()
		for {
			select {
			case <-dying:
				c.r.Debug().Msg("stopping metadata dispatcher")
				return nil
			case cb, ok := <-healthyDispatcher:
				if ok {
					cb(reporter.HealthcheckOK, "ok")
				}
			case ch := <-c.dispatcherBChannel:
				// This is to test batching
				<-ch
			case request := <-c.dispatcherChannel:
				c.dispatchIncomingRequest(request)
			}
		}
	})

	// Goroutines to poll exporters
	c.healthyWorkers = make(chan reporter.ChannelHealthcheckFunc)
	c.r.RegisterHealthcheck("metadata/worker", reporter.ChannelHealthcheck(c.t.Context(nil), c.healthyWorkers))
	for i := range c.config.Workers {
		workerIDStr := strconv.Itoa(i)
		c.t.Go(func() error {
			c.r.Debug().Str("worker", workerIDStr).Msg("starting metadata provider")
			dying := c.t.Dying()
			for {
				select {
				case <-dying:
					c.r.Debug().Str("worker", workerIDStr).Msg("stopping metadata provider")
					return nil
				case cb, ok := <-c.healthyWorkers:
					if ok {
						cb(reporter.HealthcheckOK, fmt.Sprintf("worker %s ok", workerIDStr))
					}
				case request := <-c.providerChannel:
					c.providerIncomingRequest(request)
				}
			}
		})
	}
	return nil
}

// Stop stops the metadata component
func (c *Component) Stop() error {
	defer func() {
		close(c.dispatcherChannel)
		close(c.providerChannel)
		close(c.healthyWorkers)
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

// Lookup for interface information for the provided exporter and ifIndex.
// If the information is not in the cache, it will be polled, but
// won't be returned immediately.
func (c *Component) Lookup(t time.Time, exporterIP netip.Addr, ifIndex uint) (provider.Answer, bool) {
	query := provider.Query{ExporterIP: exporterIP, IfIndex: ifIndex}
	answer, ok := c.sc.Lookup(t, query)
	if !ok {
		select {
		case c.dispatcherChannel <- query:
		default:
			c.metrics.providerBusyCount.WithLabelValues(exporterIP.Unmap().String()).Inc()
		}
	}
	return answer, ok
}

// dispatchIncomingRequest dispatches an incoming request to workers. It may
// handle more than the provided request if it can.
func (c *Component) dispatchIncomingRequest(request provider.Query) {
	requestsMap := map[netip.Addr][]uint{
		request.ExporterIP: {request.IfIndex},
	}
	dying := c.t.Dying()
	for c.config.MaxBatchRequests > 0 {
		select {
		case request := <-c.dispatcherChannel:
			indexes, ok := requestsMap[request.ExporterIP]
			if !ok {
				indexes = []uint{request.IfIndex}
			} else {
				indexes = append(indexes, request.IfIndex)
			}
			requestsMap[request.ExporterIP] = indexes
			// We don't want to exceed the configured limit but also there is no
			// point of batching requests of too many exporters.
			if len(indexes) < c.config.MaxBatchRequests && len(requestsMap) < 4 {
				continue
			}
		case <-dying:
			return
		default:
			// No more requests in queue
		}
		break
	}
	for exporterIP, ifIndexes := range requestsMap {
		if len(ifIndexes) > 1 {
			c.metrics.providerBatchedCount.Add(float64(len(ifIndexes)))
		}
		select {
		case <-dying:
			return
		case c.providerChannel <- provider.BatchQuery{ExporterIP: exporterIP, IfIndexes: ifIndexes}:
		}
	}
}

// providerIncomingRequest handles an incoming request to the provider. It
// uses a breaker to avoid pushing working on non-responsive exporters.
func (c *Component) providerIncomingRequest(request provider.BatchQuery) {
	// Avoid querying too much exporters with errors
	c.providerBreakersLock.Lock()
	providerBreaker, ok := c.providerBreakers[request.ExporterIP]
	if !ok {
		providerBreaker = breaker.New(20, 1, time.Minute)
		c.providerBreakers[request.ExporterIP] = providerBreaker
	}
	c.providerBreakersLock.Unlock()

	if err := providerBreaker.Run(func() error {
		ctx := c.t.Context(nil)
		for _, p := range c.providers {
			// Query providers in the order they are defined and stop on the
			// first provider accepting to handle the query.
			if err := p.Query(ctx, &request); err != nil && err != provider.ErrSkipProvider {
				return err
			} else if err == provider.ErrSkipProvider {
				continue
			}
			return nil
		}
		return nil
	}); err == breaker.ErrBreakerOpen {
		c.metrics.providerBreakerOpenCount.WithLabelValues(request.ExporterIP.Unmap().String()).Inc()
		c.providerBreakersLock.Lock()
		l, ok := c.providerBreakerLoggers[request.ExporterIP]
		if !ok {
			l = c.r.Sample(reporter.BurstSampler(time.Minute, 1)).
				With().
				Str("exporter", request.ExporterIP.Unmap().String()).
				Logger()
			c.providerBreakerLoggers[request.ExporterIP] = l
		}
		l.Warn().Msg("provider breaker open")
		c.providerBreakersLock.Unlock()
	}
}

// expireCache handles cache expiration and refresh.
func (c *Component) expireCache() {
	c.sc.Expire(c.d.Clock.Now().Add(-c.config.CacheDuration))
	if c.config.CacheRefresh > 0 {
		c.r.Debug().Msg("refresh metadata cache")
		c.metrics.cacheRefreshRuns.Inc()
		count := 0
		toRefresh := c.sc.NeedUpdates(c.d.Clock.Now().Add(-c.config.CacheRefresh))
		for exporter, ifaces := range toRefresh {
			for _, ifIndex := range ifaces {
				select {
				case c.dispatcherChannel <- provider.Query{
					ExporterIP: exporter,
					IfIndex:    ifIndex,
				}:
					count++
				default:
					c.metrics.providerBusyCount.WithLabelValues(exporter.Unmap().String()).Inc()
				}
			}
		}
		c.r.Debug().Int("count", count).Msg("refreshed metadata cache")
		c.metrics.cacheRefresh.Add(float64(count))
	}
}
