// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package clickhousedb

import (
	"context"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"akvorado/common/clickhousedb/mocks"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// SetupClickHouse configures a client to use for testing.
func SetupClickHouse(t *testing.T, r *reporter.Reporter, cluster bool) *Component {
	t.Helper()
	config := DefaultConfiguration()
	config.Servers = []string{
		helpers.CheckExternalService(t, "ClickHouse",
			[]string{"clickhouse:9000", "127.0.0.1:9000"})}
	if cluster {
		config.Cluster = "akvorado"
	}
	config.DialTimeout = 100 * time.Millisecond
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	c.ExecOnCluster(context.Background(), "DROP TABLE IF EXISTS system.metric_log")
	return c
}

// ClusterName returns the name of the cluster (for testing only)
func (c *Component) ClusterName() string {
	return c.config.Cluster
}

// NewMock creates a new component using a mock driver. It returns
// both the component and the mock driver.
func NewMock(t *testing.T, r *reporter.Reporter) (*Component, *mocks.MockConn) {
	t.Helper()
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	ctrl := gomock.NewController(t)
	mock := mocks.NewMockConn(ctrl)
	c.Conn = mock

	mock.EXPECT().
		Close().
		Return(nil)

	helpers.StartStop(t, c)
	return c, mock
}
