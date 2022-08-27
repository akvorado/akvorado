// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package snmp handles SNMP polling to get interface names and
// descriptions. It keeps a cache of retrieved entries and refresh
// them.
package snmp

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
)

// Component represents the SNMP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	sc *snmpCache

	healthyWorkers       chan reporter.ChannelHealthcheckFunc
	pollerChannel        chan lookupRequest
	dispatcherChannel    chan lookupRequest
	dispatcherBChannel   chan (<-chan bool) // block channel for testing
	pollerBreakersLock   sync.Mutex
	pollerBreakerLoggers map[netip.Addr]reporter.Logger
	pollerBreakers       map[netip.Addr]*breaker.Breaker
	poller               poller

	metrics struct {
		cacheRefreshRuns       reporter.Counter
		cacheRefresh           reporter.Counter
		pollerBusyCount        *reporter.CounterVec
		pollerCoalescedCount   reporter.Counter
		pollerBreakerOpenCount *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the SNMP component.
type Dependencies struct {
	Daemon daemon.Component
	Clock  clock.Clock
}

// New creates a new SNMP component.
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
	sc := newSNMPCache(r, dependencies.Clock)
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,
		sc:     sc,

		pollerChannel:        make(chan lookupRequest),
		dispatcherChannel:    make(chan lookupRequest, 100*configuration.Workers),
		dispatcherBChannel:   make(chan (<-chan bool)),
		pollerBreakers:       make(map[netip.Addr]*breaker.Breaker),
		pollerBreakerLoggers: make(map[netip.Addr]reporter.Logger),
		poller: newPoller(r, pollerConfig{
			Retries:            configuration.PollerRetries,
			Timeout:            configuration.PollerTimeout,
			Communities:        configuration.Communities,
			SecurityParameters: configuration.SecurityParameters,
		}, dependencies.Clock, sc.Put),
	}
	c.d.Daemon.Track(&c.t, "inlet/snmp")

	c.metrics.cacheRefreshRuns = r.Counter(
		reporter.CounterOpts{
			Name: "cache_refresh_runs",
			Help: "Number of times the cache refresh was triggered.",
		})
	c.metrics.cacheRefresh = r.Counter(
		reporter.CounterOpts{
			Name: "cache_refresh",
			Help: "Number of entries refreshed in cache.",
		})
	c.metrics.pollerBusyCount = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_busy_count",
			Help: "Pollers where too busy and dropped requests.",
		},
		[]string{"exporter"})
	c.metrics.pollerCoalescedCount = r.Counter(
		reporter.CounterOpts{
			Name: "poller_coalesced_count",
			Help: "Poller was able to coalesce several requests in one.",
		})
	c.metrics.pollerBreakerOpenCount = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_breaker_open_count",
			Help: "Poller breaker was opened due to too many errors.",
		},
		[]string{"exporter"})
	return &c, nil
}

// Start starts the SNMP component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting SNMP component")

	// Load cache
	if c.config.CachePersistFile != "" {
		if err := c.sc.Load(c.config.CachePersistFile); err != nil {
			c.r.Err(err).Msg("cannot load cache, ignoring")
		}
	}

	// Goroutine to refresh the cache
	healthyTicker := make(chan reporter.ChannelHealthcheckFunc)
	c.r.RegisterHealthcheck("snmp/ticker", reporter.ChannelHealthcheck(c.t.Context(nil), healthyTicker))
	c.t.Go(func() error {
		c.r.Debug().Msg("starting SNMP ticker")
		ticker := c.d.Clock.Ticker(c.config.CacheCheckInterval)
		defer ticker.Stop()
		defer close(healthyTicker)
		for {
			select {
			case <-c.t.Dying():
				c.r.Debug().Msg("shutting down SNMP ticker")
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
	c.r.RegisterHealthcheck("snmp/dispatcher", reporter.ChannelHealthcheck(c.t.Context(nil), healthyDispatcher))
	c.t.Go(func() error {
		for {
			select {
			case <-c.t.Dying():
				c.r.Debug().Msg("stopping SNMP dispatcher")
				return nil
			case cb, ok := <-healthyDispatcher:
				if ok {
					cb(reporter.HealthcheckOK, "ok")
				}
			case ch := <-c.dispatcherBChannel:
				// This is for test coaelescing
				<-ch
			case request := <-c.dispatcherChannel:
				c.dispatchIncomingRequest(request)
			}
		}
	})

	// Goroutines to poll exporters
	c.healthyWorkers = make(chan reporter.ChannelHealthcheckFunc)
	c.r.RegisterHealthcheck("snmp/worker", reporter.ChannelHealthcheck(c.t.Context(nil), c.healthyWorkers))
	for i := 0; i < c.config.Workers; i++ {
		workerIDStr := strconv.Itoa(i)
		c.t.Go(func() error {
			c.r.Debug().Str("worker", workerIDStr).Msg("starting SNMP poller")
			for {
				select {
				case <-c.t.Dying():
					c.r.Debug().Str("worker", workerIDStr).Msg("stopping SNMP poller")
					return nil
				case cb, ok := <-c.healthyWorkers:
					if ok {
						cb(reporter.HealthcheckOK, fmt.Sprintf("worker %s ok", workerIDStr))
					}
				case request := <-c.pollerChannel:
					c.pollerIncomingRequest(request)
				}
			}
		})
	}
	return nil
}

// Stop stops the SNMP component
func (c *Component) Stop() error {
	defer func() {
		close(c.dispatcherChannel)
		close(c.pollerChannel)
		close(c.healthyWorkers)
		if c.config.CachePersistFile != "" {
			if err := c.sc.Save(c.config.CachePersistFile); err != nil {
				c.r.Err(err).Msg("cannot save cache")
			}
		}
		c.r.Info().Msg("SNMP component stopped")
	}()
	c.r.Info().Msg("stopping SNMP component")
	c.t.Kill(nil)
	return c.t.Wait()
}

// lookupRequest is used internally to queue a polling request.
type lookupRequest struct {
	ExporterIP netip.Addr
	IfIndexes  []uint
}

// Lookup for interface information for the provided exporter and ifIndex.
// If the information is not in the cache, it will be polled, but
// won't be returned immediately.
func (c *Component) Lookup(exporterIP netip.Addr, ifIndex uint) (string, Interface, error) {
	exporterName, iface, err := c.sc.Lookup(exporterIP, ifIndex)
	if errors.Is(err, ErrCacheMiss) {
		req := lookupRequest{
			ExporterIP: exporterIP,
			IfIndexes:  []uint{ifIndex},
		}
		select {
		case c.dispatcherChannel <- req:
		default:
			c.metrics.pollerBusyCount.WithLabelValues(exporterIP.Unmap().String()).Inc()
		}
	}
	return exporterName, iface, err
}

// Dispatch an incoming request to workers. May handle more than the
// provided request if it can.
func (c *Component) dispatchIncomingRequest(request lookupRequest) {
	requestsMap := map[netip.Addr][]uint{
		request.ExporterIP: request.IfIndexes,
	}
	for c.config.PollerCoalesce > 0 {
		select {
		case request := <-c.dispatcherChannel:
			indexes, ok := requestsMap[request.ExporterIP]
			if !ok {
				indexes = request.IfIndexes
			} else {
				indexes = append(indexes, request.IfIndexes...)
			}
			requestsMap[request.ExporterIP] = indexes
			// We don't want to exceed the configured
			// limit but also there is no point of
			// coalescing requests of too many exporters.
			if len(indexes) < c.config.PollerCoalesce && len(requestsMap) < 4 {
				continue
			}
		case <-c.t.Dying():
			return
		default:
			// No more requests in queue
		}
		break
	}
	for exporterIP, ifIndexes := range requestsMap {
		if len(ifIndexes) > 1 {
			c.metrics.pollerCoalescedCount.Add(float64(len(ifIndexes)))
		}
		select {
		case <-c.t.Dying():
			return
		case c.pollerChannel <- lookupRequest{exporterIP, ifIndexes}:
		}
	}
}

// pollerIncomingRequest handles an incoming request to the poller. It
// uses a breaker to avoid pushing working on non-responsive exporters.
func (c *Component) pollerIncomingRequest(request lookupRequest) {
	// Avoid querying too much exporters with errors
	c.pollerBreakersLock.Lock()
	pollerBreaker, ok := c.pollerBreakers[request.ExporterIP]
	if !ok {
		pollerBreaker = breaker.New(20, 1, time.Minute)
		c.pollerBreakers[request.ExporterIP] = pollerBreaker
	}
	c.pollerBreakersLock.Unlock()

	if err := pollerBreaker.Run(func() error {
		return c.poller.Poll(
			c.t.Context(nil),
			request.ExporterIP,
			c.config.Ports.LookupOrDefault(request.ExporterIP, 161),
			request.IfIndexes)
	}); err == breaker.ErrBreakerOpen {
		c.metrics.pollerBreakerOpenCount.WithLabelValues(request.ExporterIP.Unmap().String()).Inc()
		c.pollerBreakersLock.Lock()
		l, ok := c.pollerBreakerLoggers[request.ExporterIP]
		if !ok {
			l = c.r.Sample(reporter.BurstSampler(time.Minute, 1)).
				With().
				Str("exporter", request.ExporterIP.Unmap().String()).
				Logger()
			c.pollerBreakerLoggers[request.ExporterIP] = l
		}
		l.Warn().Msg("poller breaker open")
		c.pollerBreakersLock.Unlock()
	}
}

// expireCache handles cache expiration and refresh.
func (c *Component) expireCache() {
	c.sc.Expire(c.config.CacheDuration)
	if c.config.CacheRefresh > 0 {
		c.r.Debug().Msg("refresh SNMP cache")
		c.metrics.cacheRefreshRuns.Inc()
		count := 0
		toRefresh := c.sc.NeedUpdates(c.config.CacheRefresh)
		for exporter, ifaces := range toRefresh {
			for ifIndex := range ifaces {
				select {
				case c.dispatcherChannel <- lookupRequest{
					ExporterIP: exporter,
					IfIndexes:  []uint{ifIndex},
				}:
					count++
				default:
					c.metrics.pollerBusyCount.WithLabelValues(exporter.Unmap().String()).Inc()
				}
			}
		}
		c.r.Debug().Int("count", count).Msg("refreshed SNMP cache")
		c.metrics.cacheRefresh.Add(float64(count))
	}
}
