// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse handles configuration of the ClickHouse database.
package clickhouse

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"akvorado/common/remotedatasourcefetcher"

	"github.com/cenkalti/backoff/v4"
	"github.com/kentik/patricia"
	"gopkg.in/tomb.v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/orchestrator/clickhouse/geoip"
)

// Component represents the ClickHouse configurator.
type Component struct {
	r       *reporter.Reporter
	d       *Dependencies
	t       tomb.Tomb
	config  Configuration
	metrics metrics

	migrationsDone        chan bool // closed when migrations are done
	migrationsOnce        chan bool // closed after first attempt to migrate
	networkSourcesFetcher *remotedatasourcefetcher.Component[externalNetworkAttributes]
	networkSources        map[string][]externalNetworkAttributes
	networkSourcesLock    sync.RWMutex
	geoipSources          map[string]*helpers.SubnetMap[NetworkAttributes]
	geoipOrder            map[string]int
	geoipSourcesLock      sync.RWMutex
	convergedNetworksLock sync.RWMutex
	convergedNetworks     *helpers.SubnetMap[NetworkAttributes]
}

// Dependencies define the dependencies of the ClickHouse configurator.
type Dependencies struct {
	Daemon     daemon.Component
	HTTP       *httpserver.Component
	ClickHouse *clickhousedb.Component
	Schema     *schema.Component
	GeoIP      *geoip.Component
}

// New creates a new ClickHouse component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:                 r,
		d:                 &dependencies,
		config:            configuration,
		migrationsDone:    make(chan bool),
		migrationsOnce:    make(chan bool),
		networkSources:    make(map[string][]externalNetworkAttributes),
		geoipSources:      make(map[string]*helpers.SubnetMap[NetworkAttributes]),
		geoipOrder:        make(map[string]int),
		convergedNetworks: helpers.MustNewSubnetMap[NetworkAttributes](nil),
	}
	var err error
	c.networkSourcesFetcher, err = remotedatasourcefetcher.New[externalNetworkAttributes](r, c.UpdateRemoteDataSource, "network_source", configuration.NetworkSources)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize remote data source fetcher component: %w", err)
	}
	c.initMetrics()

	if err := c.registerHTTPHandlers(); err != nil {
		return nil, err
	}

	// Ensure resolutions are sorted and we have a 0-interval resolution first.
	sort.Slice(c.config.Resolutions, func(i, j int) bool {
		return c.config.Resolutions[i].Interval < c.config.Resolutions[j].Interval
	})
	if len(c.config.Resolutions) == 0 || c.config.Resolutions[0].Interval != 0 {
		return nil, fmt.Errorf("resolutions need to be configured, including interval: 0")
	}

	c.d.Daemon.Track(&c.t, "orchestrator/clickhouse")

	return &c, nil
}

// Start the ClickHouse component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting ClickHouse component")

	// stub to prevent tomb dying immediately after migrations are done
	c.t.Go(func() error {
		<-c.t.Dying()
		return nil
	})

	// Database migration
	migrationsOnce := false
	c.metrics.migrationsRunning.Set(1)
	c.t.Go(func() error {
		customBackoff := backoff.NewExponentialBackOff()
		customBackoff.MaxElapsedTime = 0
		customBackoff.InitialInterval = time.Second
		for {
			if !c.config.SkipMigrations {
				c.r.Info().Msg("attempting database migration")
				if err := c.migrateDatabase(); err != nil {
					c.r.Err(err).Msg("database migration error")
				} else {
					return nil
				}
				if !migrationsOnce {
					close(c.migrationsOnce)
					migrationsOnce = true
					customBackoff.Reset()
				}
			}
			next := customBackoff.NextBackOff()
			select {
			case <-c.t.Dying():
				return nil
			case <-time.Tick(next):
			}
		}
	})

	// refresh converged networks after migrations
	// because it will trigger a RELOAD SYSTEM DICTIONARY

	// not sure here if c.migrationsDone should be closed
	// regardless of wether migrations are skipped or not
	if !c.config.SkipMigrations {
		<-c.migrationsDone
	}
	c.r.Log().Msg("refreshing converved networks")
	if err := c.refreshConvergedNetworks(); err != nil {
		return err
	}

	// Network sources update
	if err := c.networkSourcesFetcher.Start(); err != nil {
		return fmt.Errorf("unable to start network sources fetcher component: %w", err)
	}
	notifyChan, initDoneChan := c.d.GeoIP.Notify()

	// geoip process updates
	c.t.Go(func() error {
		c.r.Log().Msg("Starting geoip refresher")
		for {
			select {
			case <-c.t.Dying():
				return nil
			case notif := <-notifyChan:
				geoipData := helpers.MustNewSubnetMap[NetworkAttributes](nil)
				switch notif.Kind {
				case "asn":
					err := c.d.GeoIP.IterASNDatabase(notif.Path, func(subnet *net.IPNet, data geoip.ASNInfo) error {
						subV6Str, err := helpers.SubnetMapParseKey(subnet.String())
						if err != nil {
							return err
						}
						attrs := NetworkAttributes{
							ASN:    data.ASNumber,
							Tenant: data.ASName,
						}
						return geoipData.Update(subV6Str, attrs, overrideNetworkAttrs(attrs))
					})
					if err != nil {
						return err
					}
				case "geo":
					err := c.d.GeoIP.IterGeoDatabase(notif.Path, func(subnet *net.IPNet, data geoip.GeoInfo) error {
						subV6Str, err := helpers.SubnetMapParseKey(subnet.String())
						if err != nil {
							return err
						}
						attrs := NetworkAttributes{
							State:   data.State,
							Country: data.Country,
							City:    data.City,
						}
						return geoipData.Update(subV6Str, attrs, overrideNetworkAttrs(attrs))
					})
					if err != nil {
						return err
					}
				}
				c.geoipSourcesLock.Lock()
				c.geoipSources[notif.Path] = geoipData
				c.geoipOrder[notif.Path] = notif.Index
				c.geoipSourcesLock.Unlock()
			}
			if err := c.refreshConvergedNetworks(); err != nil {
				return err
			}
		}
	})

	// wait for initial sync of geoip component
	select {
	case <-initDoneChan:
	case <-c.t.Dying():
	}
	c.r.Info().Msg("ClickHouse component started")
	return nil
}

func overrideNetworkAttrs(newAttrs NetworkAttributes) func(existing NetworkAttributes) NetworkAttributes {
	return func(existing NetworkAttributes) NetworkAttributes {
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
			existing.Site = newAttrs.Role
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
}

func (c *Component) refreshConvergedNetworks() error {

	c.geoipSourcesLock.RLock()
	// inject info from GeoIP first so that custom networks will override
	networks := helpers.MustNewSubnetMap[NetworkAttributes](nil)
	// do the iteration in the order of the configured database in the configuration
	geoipDbs := make([]string, 0, len(c.geoipSources))
	for k := range c.geoipOrder {
		geoipDbs = append(geoipDbs, k)
	}
	sort.Slice(geoipDbs, func(i, j int) bool {
		// sort in reverse order, so that the first item of the user list overrides the data (first=best)
		return c.geoipOrder[geoipDbs[i]] > c.geoipOrder[geoipDbs[j]]
	})

	for _, dbName := range geoipDbs {
		err := c.geoipSources[dbName].Iter(func(address patricia.IPv6Address, tags [][]NetworkAttributes) error {
			return networks.Update(
				address.String(),
				tags[len(tags)-1][0],
				// override existing network attributes
				overrideNetworkAttrs(tags[len(tags)-1][0]),
			)
		})
		if err != nil {
			return err
		}
	}
	c.geoipSourcesLock.RUnlock()

	c.networkSourcesLock.RLock()
	for _, networkList := range c.networkSources {
		for _, val := range networkList {
			if err := networks.Update(
				val.Prefix.String(),
				val.NetworkAttributes,
				// override existing network attributes
				overrideNetworkAttrs(val.NetworkAttributes),
			); err != nil {
				return err
			}
		}
	}
	c.networkSourcesLock.RUnlock()
	if c.config.Networks != nil {
		err := c.config.Networks.Iter(func(address patricia.IPv6Address, tags [][]NetworkAttributes) error {
			return networks.Update(
				address.String(),
				tags[len(tags)-1][0],
				// override existing network attributes
				overrideNetworkAttrs(tags[len(tags)-1][0]),
			)
		})
		if err != nil {
			return err
		}
	}

	c.convergedNetworksLock.Lock()
	c.convergedNetworks = networks
	c.convergedNetworksLock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.ReloadDictionary(ctx, schema.DictionaryNetworks); err != nil {
		c.r.Err(err).Msg("failed to refresh networks dict")
	}
	return nil
}

// Stop stops the ClickHouse component.
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping ClickHouse component")
	defer c.r.Info().Msg("ClickHouse component stopped")
	c.t.Kill(nil)
	return c.t.Wait()
}
