// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package networks handles network attribute lookups for the outlet enricher.
// The attributes are gathered from the configuration, from remote sources and
// from the GeoIP databases, then merged into a single set of prefixes.
package networks

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gaissmai/bart"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/common/reporter"
	"akvorado/outlet/geoip"
)

// Component represents the networks component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	networkSourcesFetcher *remotedatasource.Component[externalNetworkAttributes]
	networkSources        map[string][]externalNetworkAttributes
	// rebuildLock guards networkSources and serializes the rebuilds. It is held
	// for the whole rebuild, otherwise two sources refreshing at the same time
	// could publish a tree missing the changes of the other one.
	rebuildLock sync.Mutex
	// rebuildWanted tells a rebuild is needed. It is set before taking
	// rebuildLock and cleared while holding it, so a caller waiting on the lock
	// knows if the rebuild it was asking for was already done by another one.
	rebuildWanted atomic.Bool
	// geoipUpdate receives a notification when a GeoIP database changes.
	geoipUpdate <-chan struct{}

	// networks is replaced on each rebuild. Lookups only load it.
	networks atomic.Pointer[bart.Fast[NetworkAttributes]]

	metrics metrics
}

// ancestor is a prefix containing the one currently flattened, with the
// attributes it inherited itself.
type ancestor struct {
	prefix     netip.Prefix
	attributes NetworkAttributes
}

type externalNetworkAttributes struct {
	Prefix            netip.Prefix
	NetworkAttributes `mapstructure:",squash"`
}

// Dependencies define the dependencies of the networks component.
type Dependencies struct {
	Daemon daemon.Component
	GeoIP  *geoip.Component
}

// New creates a new networks component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:              r,
		d:              &dependencies,
		config:         configuration,
		networkSources: make(map[string][]externalNetworkAttributes),
	}
	c.networks.Store(&bart.Fast[NetworkAttributes]{})
	var err error
	c.networkSourcesFetcher, err = remotedatasource.New[externalNetworkAttributes](
		r, c.UpdateSource, "network_source", configuration.NetworkSources)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize remote data source fetcher component: %w", err)
	}
	if c.d.GeoIP != nil {
		c.geoipUpdate = c.d.GeoIP.Notify()
	}
	dependencies.Daemon.Track(&c.t, "outlet/networks")
	c.initMetrics()
	return &c, nil
}

// Start starts the networks component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting networks component")

	c.t.Go(func() error {
		<-c.t.Dying()
		return nil
	})

	// Build the initial tree from the GeoIP databases and the static
	// configuration. Databases opened on start have already sent a
	// notification, drop it as we are reading them right now.
	if c.geoipUpdate != nil {
		select {
		case <-c.geoipUpdate:
		default:
		}
	}
	c.rebuild()

	if c.geoipUpdate != nil {
		c.t.Go(func() error {
			for {
				select {
				case <-c.t.Dying():
					return nil
				case <-c.geoipUpdate:
					c.rebuild()
				}
			}
		})
	}

	if err := c.networkSourcesFetcher.Start(); err != nil {
		return fmt.Errorf("unable to start network sources fetcher component: %w", err)
	}

	// Give the remote sources a chance to be fetched before flows are enriched
	if len(c.config.NetworkSources) > 0 && c.config.NetworkSourcesTimeout > 0 {
		timer := time.NewTimer(c.config.NetworkSourcesTimeout)
		defer timer.Stop()
		select {
		case <-c.networkSourcesFetcher.DataSourcesReady:
		case <-c.t.Dying():
		case <-timer.C:
			c.r.Warn().Msg("network sources not ready, continuing without them")
		}
	}

	c.r.Info().Msg("networks component started")
	return nil
}

// Stop stops the networks component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping networks component")
	defer c.r.Info().Msg("networks component stopped")
	c.t.Kill(nil)
	c.networkSourcesFetcher.Stop()
	return c.t.Wait()
}

// UpdateSource updates a remote network source. It returns the
// number of networks retrieved.
func (c *Component) UpdateSource(ctx context.Context, name string, source remotedatasource.Source) (int, error) {
	results, err := c.networkSourcesFetcher.Fetch(ctx, name, source)
	if err != nil {
		return 0, err
	}
	c.updateSource(name, results)
	return len(results), nil
}

// updateSource publishes the attributes fetched from a remote source and
// rebuilds. The attributes are published while holding rebuildLock, otherwise a
// rebuild running concurrently could miss them and this call would still be
// considered as covered by it.
//
// Sources are refetched on each interval, most of the time without any change.
// A rebuild walks the whole GeoIP databases, therefore it is only done when the
// content differs from the one the current tree was built with.
func (c *Component) updateSource(name string, results []externalNetworkAttributes) {
	c.rebuildLock.Lock()
	unchanged := slices.Equal(c.networkSources[name], results)
	c.networkSources[name] = results
	c.rebuildLock.Unlock()
	if unchanged {
		return
	}
	c.rebuild()
}

// Lookup looks up the network attributes for the given IP address. The
// attributes of the most specific prefix win.
func (c *Component) Lookup(ip netip.Addr) NetworkAttributes {
	attributes, _ := c.networks.Load().Lookup(ip)
	return attributes
}

// rebuild rebuilds the whole tree from the GeoIP databases, the remote sources
// and the static configuration. When several of them provide the same prefix,
// the attributes are merged, the last source winning: GeoIP, then the remote
// sources, then the static configuration.
//
// Walking the GeoIP databases is expensive, therefore concurrent calls are
// coalesced: a caller returns as soon as a rebuild covering its own changes is
// done, which is not necessarily the one it triggered. Callers must publish
// their changes before calling.
func (c *Component) rebuild() {
	c.rebuildWanted.Store(true)
	c.rebuildLock.Lock()
	defer c.rebuildLock.Unlock()
	if !c.rebuildWanted.Swap(false) {
		// Another call took the lock first and its rebuild already saw our
		// changes, as we published them before asking for a rebuild.
		return
	}
	c.r.Debug().Msg("rebuild networks")
	start := time.Now()

	// The merged tree is only walked, never looked up, so it does not need the
	// lookup-optimized flavor of the tree.
	merged := &bart.Table[NetworkAttributes]{}
	update := func(prefix netip.Prefix, attributes NetworkAttributes) {
		merged.Modify(helpers.PrefixTo6(prefix),
			func(existing NetworkAttributes, _ bool) (NetworkAttributes, bool) {
				return mergeNetworkAttrs(existing, attributes), false
			})
	}

	// Add the content of the GeoIP databases
	if c.d.GeoIP != nil {
		c.d.GeoIP.IterASNDatabases(func(prefix netip.Prefix, data geoip.ASNInfo) {
			update(prefix, NetworkAttributes{ASN: data.ASNumber})
		})
		c.d.GeoIP.IterGeoDatabases(func(prefix netip.Prefix, data geoip.GeoInfo) {
			update(prefix, NetworkAttributes{
				State:   data.State,
				Country: data.Country,
				City:    data.City,
			})
		})
	}

	// Add the remote network sources
	for _, networkList := range c.networkSources {
		for _, val := range networkList {
			update(val.Prefix, val.NetworkAttributes)
		}
	}

	// Add the static networks
	if c.config.Networks != nil {
		for prefix, attributes := range c.config.Networks.All() {
			update(prefix, attributes)
		}
	}

	// Merge the attributes from the least specific prefixes into the most
	// specific ones: a prefix inherits the attributes it does not define
	// itself. AllSorted() walks the prefixes in CIDR order, therefore a prefix
	// always comes after the ones containing it and ancestors keeps the chain
	// of prefixes containing the current one. Its last element already
	// inherited from the rest of the chain, merging with it is enough.
	flattened := &bart.Fast[NetworkAttributes]{}
	ancestors := make([]ancestor, 0, 128)
	for prefix, attributes := range merged.AllSorted() {
		for len(ancestors) > 0 && !ancestors[len(ancestors)-1].prefix.Contains(prefix.Addr()) {
			ancestors = ancestors[:len(ancestors)-1]
		}
		if len(ancestors) > 0 {
			attributes = mergeNetworkAttrs(ancestors[len(ancestors)-1].attributes, attributes)
		}
		ancestors = append(ancestors, ancestor{prefix, attributes})
		flattened.Insert(prefix, attributes)
	}

	c.networks.Store(flattened)
	elapsed := time.Since(start).Seconds()
	c.metrics.rebuilds.Inc()
	c.metrics.rebuildTime.Add(elapsed)
	c.metrics.rebuildLastTime.Set(elapsed)
	c.metrics.prefixes.Set(float64(flattened.Size()))
}

// mergeNetworkAttrs overrides the attributes set in newAttrs into existing.
func mergeNetworkAttrs(existing, newAttrs NetworkAttributes) NetworkAttributes {
	if newAttrs.ASN != 0 {
		existing.ASN = newAttrs.ASN
	}
	if newAttrs.Name != "" {
		existing.Name = newAttrs.Name
	}
	if newAttrs.Region != "" {
		existing.Region = newAttrs.Region
	}
	if newAttrs.Site != "" {
		existing.Site = newAttrs.Site
	}
	if newAttrs.Role != "" {
		existing.Role = newAttrs.Role
	}
	if newAttrs.Tenant != "" {
		existing.Tenant = newAttrs.Tenant
	}
	if newAttrs.Country != "" {
		existing.Country = newAttrs.Country
	}
	if newAttrs.State != "" {
		existing.State = newAttrs.State
	}
	if newAttrs.City != "" {
		existing.City = newAttrs.City
	}
	return existing
}
