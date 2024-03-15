// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"net/netip"

	"akvorado/common/remotedatasourcefetcher"
)

type externalNetworkAttributes struct {
	Prefix            netip.Prefix
	NetworkAttributes `mapstructure:",squash"`
}

// UpdateRemoteDataSource updates a remote network source. It returns the
// number of networks retrieved.
func (c *Component) UpdateRemoteDataSource(ctx context.Context, name string, source remotedatasourcefetcher.RemoteDataSource) (int, error) {
	results, err := c.networkSourcesFetcher.Fetch(ctx, name, source)
	if err != nil {
		return 0, err
	}
	c.networkSourcesLock.Lock()
	c.networkSources[name] = results
	c.networkSourcesLock.Unlock()
	c.refreshNetworkDictionary()
	return len(results), nil
}

func overrideNetworkAttrs(newAttrs NetworkAttributes) func(existing NetworkAttributes) NetworkAttributes {
	return func(existing NetworkAttributes) NetworkAttributes {
		return mergeNetworkAttrs(existing, newAttrs)
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
