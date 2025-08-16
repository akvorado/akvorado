// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/orchestrator/geoip"
)

const networksCSVPattern = "networks*.csv.gz"

func (c *Component) refreshNetworksCSV() {
	select {
	case c.networksCSVUpdateChan <- true:
	default:
	}
}

func (c *Component) networksCSVRefresher() {
	// Wait for network sources to be ready
	select {
	case <-c.t.Dying():
		return
	case <-c.networkSourcesFetcher.DataSourcesReady:
	}
	// Wait for migrations
	if !c.config.SkipMigrations {
		select {
		case <-c.t.Dying():
			return
		case <-c.migrationsDone:
		}
	}

	once := true
	for {
		select {
		case <-c.t.Dying():
			return
		case <-c.networksCSVUpdateChan:
		}

		c.r.Debug().Msg("build networks.csv")
		networks := helpers.MustNewSubnetMap[NetworkAttributes](nil)

		// Add content of all geoip databases
		err := c.d.GeoIP.IterASNDatabases(func(prefix netip.Prefix, data geoip.ASNInfo) error {
			subV6Prefix := helpers.PrefixTo16(prefix)
			attrs := NetworkAttributes{
				ASN: data.ASNumber,
			}
			networks.Update(subV6Prefix, func(existing NetworkAttributes, _ bool) NetworkAttributes {
				return mergeNetworkAttrs(existing, attrs)
			})
			return nil
		})
		if err != nil {
			c.r.Err(err).Msg("unable to iter over ASN databases")
			return
		}
		err = c.d.GeoIP.IterGeoDatabases(func(prefix netip.Prefix, data geoip.GeoInfo) error {
			subV6Prefix := helpers.PrefixTo16(prefix)
			attrs := NetworkAttributes{
				State:   data.State,
				Country: data.Country,
				City:    data.City,
			}
			networks.Update(subV6Prefix, func(existing NetworkAttributes, _ bool) NetworkAttributes {
				return mergeNetworkAttrs(existing, attrs)
			})
			return nil
		})
		if err != nil {
			c.r.Err(err).Msg("unable to iter over geo databases")
			return
		}
		// Add network sources
		if err := func() error {
			c.networkSourcesLock.RLock()
			defer c.networkSourcesLock.RUnlock()
			for _, networkList := range c.networkSources {
				for _, val := range networkList {
					subV6Prefix := helpers.PrefixTo16(val.Prefix)
					networks.Update(subV6Prefix, func(existing NetworkAttributes, _ bool) NetworkAttributes {
						return mergeNetworkAttrs(existing, val.NetworkAttributes)
					})
				}
			}
			return nil
		}(); err != nil {
			c.r.Err(err).Msg("unable to update with remote network sources")
			return
		}
		// Add static network sources
		if c.config.Networks != nil {
			// Update networks with static network source
			for prefix, attrs := range c.config.Networks.All() {
				networks.Update(prefix, func(existing NetworkAttributes, _ bool) NetworkAttributes {
					return mergeNetworkAttrs(existing, attrs)
				})
			}
		}

		// Clean up old files
		oldFiles, err := filepath.Glob(filepath.Join(os.TempDir(), networksCSVPattern))
		if err == nil {
			for _, oldFile := range oldFiles {
				os.Remove(oldFile)
			}
		}

		// Create a temporary file to hold results
		tmpfile, err := os.CreateTemp("", networksCSVPattern)
		if err != nil {
			c.r.Err(err).Msg("cannot create temporary file for networks.csv")
			return
		}

		// Write a gzip dump to the disk
		gzipWriter := gzip.NewWriter(tmpfile)
		csvWriter := csv.NewWriter(gzipWriter)
		csvWriter.Write([]string{"network", "name", "role", "site", "region", "country", "state", "city", "tenant", "asn"})
		for prefix, leafAttrs := range networks.AllMaybeSorted() {
			// Merge attributes from root to leaf for hierarchical inheritance.
			// Supernets() returns in reverse-CIDR order (LPM to root), so we
			// merge in that order.
			current := leafAttrs
			for _, attrs := range networks.Supernets(prefix) {
				current = mergeNetworkAttrs(attrs, current)
			}

			var asnVal string
			if current.ASN != 0 {
				asnVal = strconv.Itoa(int(current.ASN))
			}
			csvWriter.Write([]string{
				prefix.String(),
				current.Name,
				current.Role,
				current.Site,
				current.Region,
				current.Country,
				current.State,
				current.City,
				current.Tenant,
				asnVal,
			})
		}
		csvWriter.Flush()
		gzipWriter.Close()

		c.networksCSVLock.Lock()
		if c.networksCSVFile != nil {
			c.networksCSVFile.Close()
			os.Remove(c.networksCSVFile.Name())
		}
		c.networksCSVFile = tmpfile
		c.networksCSVLock.Unlock()

		if once {
			close(c.networksCSVReady)
			once = false
		}

		func() {
			ctx, cancel := context.WithTimeout(c.t.Context(nil), time.Minute)
			defer cancel()
			c.metrics.networksReload.Inc()
			if err := c.ReloadDictionary(ctx, schema.DictionaryNetworks); err != nil {
				c.r.Err(err).Msg("failed to refresh networks dictionary")
			}
		}()
	}
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
