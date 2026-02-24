// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package networks handles network attribute lookups for the outlet enricher.
package networks

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/remotedatasource"
	"akvorado/common/reporter"
)

// Component represents the networks component.
type Component struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config Configuration

	networkSourcesFetcher *remotedatasource.Component[externalNetworkAttributes]
	networkSources        map[string][]externalNetworkAttributes
	// networkSourcesLock guards networkSources and serializes the rebuilds. It
	// is held for the whole rebuild, otherwise two sources refreshing at the
	// same time could publish a map missing the changes of the other one.
	networkSourcesLock sync.Mutex

	// merged is replaced on each rebuild. Lookups only load it.
	merged atomic.Pointer[helpers.SubnetMap[NetworkAttributes]]
}

type externalNetworkAttributes struct {
	Prefix            netip.Prefix
	NetworkAttributes `mapstructure:",squash"`
}

// Dependencies define the dependencies of the networks component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new networks component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:              r,
		config:         configuration,
		networkSources: make(map[string][]externalNetworkAttributes),
	}
	c.merged.Store(helpers.MustNewSubnetMap[NetworkAttributes](nil))
	var err error
	c.networkSourcesFetcher, err = remotedatasource.New[externalNetworkAttributes](
		r, c.UpdateSource, "network_source", configuration.NetworkSources)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize remote data source fetcher component: %w", err)
	}
	dependencies.Daemon.Track(&c.t, "outlet/networks")
	return &c, nil
}

// Start starts the networks component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting networks component")

	c.t.Go(func() error {
		<-c.t.Dying()
		return nil
	})

	// Build the initial map from the static configuration only
	c.networkSourcesLock.Lock()
	c.rebuildMerged()
	c.networkSourcesLock.Unlock()

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
	c.networkSourcesLock.Lock()
	defer c.networkSourcesLock.Unlock()
	c.networkSources[name] = results
	c.rebuildMerged()
	return len(results), nil
}

// Lookup looks up network attributes for the given IP address, merging
// attributes from supernets for hierarchical inheritance.
func (c *Component) Lookup(ip netip.Addr) NetworkAttributes {
	return c.merged.Load().LookupOrDefault(ip, NetworkAttributes{})
}

// rebuildMerged rebuilds the merged SubnetMap from remote sources and static
// config. The caller must hold networkSourcesLock.
func (c *Component) rebuildMerged() {
	c.r.Debug().Msg("rebuilding merged networks map")
	networks := helpers.MustNewSubnetMap[NetworkAttributes](nil)

	// Add network sources
	for _, networkList := range c.networkSources {
		for _, val := range networkList {
			subV6Prefix := helpers.PrefixTo6(val.Prefix)
			networks.Update(subV6Prefix, func(existing NetworkAttributes, _ bool) NetworkAttributes {
				return mergeNetworkAttrs(existing, val.NetworkAttributes)
			})
		}
	}

	// Add static network sources (override remote)
	if c.config.Networks != nil {
		for prefix, attrs := range c.config.Networks.All() {
			networks.Update(prefix, func(existing NetworkAttributes, _ bool) NetworkAttributes {
				return mergeNetworkAttrs(existing, attrs)
			})
		}
	}

	// Flatten hierarchical inheritance: for each entry, merge attributes from supernets
	flattened := helpers.MustNewSubnetMap[NetworkAttributes](nil)
	for prefix, leafAttrs := range networks.All() {
		current := leafAttrs
		for _, attrs := range networks.Supernets(prefix) {
			current = mergeNetworkAttrs(attrs, current)
		}
		flattened.Set(prefix, current)
	}

	c.merged.Store(flattened)
}

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
