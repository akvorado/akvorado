// Package snmp handles SNMP polling to get interface names and
// descriptions. It keeps a cache of retrieved entries and refresh
// them.
package snmp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/benbjohnson/clock"
	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/reporter"
)

// Component represents the SNMP compomenent.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	sc *snmpCache

	healthyWorkers chan reporter.ChannelHealthcheckFunc
	pollerChannel  chan lookupRequest
	poller         poller

	metrics struct {
		cacheRefreshRuns reporter.Counter
		cacheRefresh     reporter.Counter
		pollerLoopTime   *reporter.SummaryVec
		pollerBusyCount  *reporter.CounterVec
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

		pollerChannel: make(chan lookupRequest, 100*configuration.Workers),
		poller:        newPoller(r, dependencies.Clock, sc.Put),
	}
	c.d.Daemon.Track(&c.t, "snmp")

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
	c.metrics.pollerLoopTime = r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "poller_loop_time_seconds",
			Help:       "Time spent in each state of the poller loop.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"worker", "state"})
	c.metrics.pollerBusyCount = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_busy_count",
			Help: "Pollers where too busy and dropped requests.",
		},
		[]string{"sampler"})
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
			case cb := <-healthyTicker:
				if cb != nil {
					cb(reporter.HealthcheckOK, "ok")
				}
			case <-ticker.C:
				c.sc.Expire(c.config.CacheDuration)
				if c.config.CacheRefresh > 0 {
					c.r.Debug().Msg("refresh SNMP cache")
					c.metrics.cacheRefreshRuns.Inc()
					count := 0
					toRefresh := c.sc.NeedUpdates(c.config.CacheRefresh)
					for sampler, ifaces := range toRefresh {
						for ifIndex := range ifaces {
							c.pollerChannel <- lookupRequest{
								SamplerIP: sampler,
								IfIndex:   ifIndex,
							}
							count++
						}
					}
					c.r.Debug().Int("count", count).Msg("refreshed SNMP cache")
					c.metrics.cacheRefresh.Add(float64(count))
				}
			}
		}
	})

	// Goroutines to poll samplers
	c.healthyWorkers = make(chan reporter.ChannelHealthcheckFunc)
	c.r.RegisterHealthcheck("snmp/worker", reporter.ChannelHealthcheck(c.t.Context(nil), c.healthyWorkers))
	for i := 0; i < c.config.Workers; i++ {
		workerIDStr := strconv.Itoa(i)
		c.t.Go(func() error {
			c.r.Debug().Str("worker", workerIDStr).Msg("starting SNMP poller")
			for {
				startIdle := time.Now()
				select {
				case <-c.t.Dying():
					c.r.Debug().Str("worker", workerIDStr).Msg("stopping SNMP poller")
					return nil
				case cb := <-c.healthyWorkers:
					if cb != nil {
						cb(reporter.HealthcheckOK, fmt.Sprintf("worker %s ok", workerIDStr))
					}
				case request := <-c.pollerChannel:
					startBusy := time.Now()
					samplerIP := request.SamplerIP
					ifIndex := request.IfIndex
					community, ok := c.config.Communities[samplerIP]
					if !ok {
						community = c.config.DefaultCommunity
					}
					c.poller.Poll(
						c.t.Context(context.Background()),
						samplerIP, 161,
						community,
						ifIndex)
					idleTime := float64(startBusy.Sub(startIdle).Nanoseconds()) / 1000 / 1000 / 1000
					busyTime := float64(time.Since(startBusy).Nanoseconds()) / 1000 / 1000 / 1000
					c.metrics.pollerLoopTime.WithLabelValues(workerIDStr, "idle").Observe(idleTime)
					c.metrics.pollerLoopTime.WithLabelValues(workerIDStr, "busy").Observe(busyTime)
				}
			}
		})
	}
	return nil
}

// Stop stops the SNMP component
func (c *Component) Stop() error {
	defer func() {
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
	SamplerIP string
	IfIndex   uint
}

// Lookup for interface information for the provided sampler and ifIndex.
// If the information is not in the cache, it will be polled, but
// won't be returned immediately.
func (c *Component) Lookup(samplerIP string, ifIndex uint) (string, Interface, error) {
	samplerName, iface, err := c.sc.Lookup(samplerIP, ifIndex)
	if errors.Is(err, ErrCacheMiss) {
		req := lookupRequest{
			SamplerIP: samplerIP,
			IfIndex:   ifIndex,
		}
		select {
		case c.pollerChannel <- req:
		default:
			c.metrics.pollerBusyCount.WithLabelValues(samplerIP).Inc()
		}
	}
	return samplerName, iface, err
}
