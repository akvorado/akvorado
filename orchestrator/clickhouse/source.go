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
	c.refreshNetworksCSV()
	return len(results), nil
}
