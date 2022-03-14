// Package snmp handles SNMP polling to get interface names and
// descriptions. It keeps a cache of retrieved entries and refresh
// them.
package snmp

import (
	"context"
	"errors"

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

	pollerChannel chan lookupRequest
	poller        poller

	metrics struct {
		cacheRefreshRuns reporter.Counter
		cacheRefresh     reporter.Counter
	}
}

// Dependencies define the dependencies of the SNMP component.
type Dependencies struct {
	Daemon daemon.Component
	Clock  clock.Clock
}

// New creates a new SNMP component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	sc := newSNMPCache(r, dependencies.Clock)
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,
		sc:     sc,

		pollerChannel: make(chan lookupRequest, 10*configuration.Workers),
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
	c.t.Go(func() error {
		c.r.Debug().Msg("starting SNMP ticker")
		ticker := c.d.Clock.Ticker(c.config.CacheRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.t.Dying():
				c.r.Debug().Msg("shutting down SNMP ticker")
				if c.config.CachePersistFile != "" {
					if err := c.sc.Save(c.config.CachePersistFile); err != nil {
						c.r.Err(err).Msg("cannot save cache")
					}
				}
				return nil
			case <-ticker.C:
				c.sc.Expire(c.config.CacheDuration)
				if c.config.CacheRefresh > 0 {
					c.r.Debug().Msg("refresh SNMP cache")
					c.metrics.cacheRefreshRuns.Inc()
					count := 0
					threshold := c.config.CacheDuration - c.config.CacheRefresh
					toRefresh := c.sc.WouldExpire(threshold)
					for sampler, ifaces := range toRefresh {
						for ifIndex := range ifaces {
							c.pollerChannel <- lookupRequest{
								Sampler: sampler,
								IfIndex: ifIndex,
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
	for i := 0; i < c.config.Workers; i++ {
		workerID := i
		c.t.Go(func() error {
			c.r.Debug().Int("worker", workerID).Msg("starting SNMP poller")
			for {
				select {
				case <-c.t.Dying():
					c.r.Debug().Int("worker", workerID).Msg("stopping SNMP poller")
					return nil
				case request := <-c.pollerChannel:
					sampler := request.Sampler
					ifIndex := request.IfIndex
					community, ok := c.config.Communities[sampler]
					if !ok {
						community = c.config.DefaultCommunity
					}
					c.poller.Poll(
						c.t.Context(context.Background()),
						sampler, 161,
						community,
						ifIndex)
				}
			}
		})
	}
	return nil
}

// Stop stops the SNMP component
func (c *Component) Stop() error {
	defer close(c.pollerChannel)
	c.r.Info().Msg("stopping SNMP component")
	defer c.r.Info().Msg("SNMP component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}

// lookupRequest is used internally to queue a polling request.
type lookupRequest struct {
	Sampler string
	IfIndex uint
}

// Lookup for interface information for the provided sampler and ifIndex.
// If the information is not in the cache, it will be polled, but
// won't be returned immediately.
func (c *Component) Lookup(sampler string, ifIndex uint) (Interface, error) {
	iface, err := c.sc.Lookup(sampler, ifIndex)
	if errors.Is(err, ErrCacheMiss) {
		c.pollerChannel <- lookupRequest{
			Sampler: sampler,
			IfIndex: ifIndex,
		}
	}
	return iface, err
}
