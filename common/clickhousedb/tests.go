// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package clickhousedb

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/mock/gomock"

	"akvorado/common/clickhousedb/mocks"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	sb "akvorado/common/sqlbuilder"
)

// SetupClickHouseDatabase creates a temporary ClickHouse database and returns
// the server address and database name. The database is dropped when the test ends.
func SetupClickHouseDatabase(t *testing.T) (server, database string) {
	t.Helper()
	server = helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse:9000", "127.0.0.1:9000"})
	database = setupClickHouseDatabase(t, []string{server}, "")
	return server, database
}

func setupClickHouseDatabase(t *testing.T, servers []string, cluster string) string {
	t.Helper()
	database := fmt.Sprintf("test_%x", rand.Uint64())
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:        servers,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("clickhouse.Open() error:\n%+v", err)
	}
	for _, statement := range []sb.Statement{
		sb.CreateDatabase(database),
		sb.DropTable(sb.Table("metric_log").In("system")),
	} {
		query := onClusterSQL(statement, cluster)
		if err := conn.Exec(t.Context(), query); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", query, err)
		}
	}
	t.Cleanup(func() {
		defer conn.Close()
		// This is important to NOT use t.Context() as it is already cancelled
		// at this point.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn.Exec(ctx, onClusterSQL(sb.DropDatabase(database), cluster))
	})
	return database
}

// Servers returns the addresses of the ClickHouse servers the component talks
// to. A query only goes to one of them, so this is for a test needing to reach
// each server of a cluster on its own.
func (c *Component) Servers() []string {
	return c.config.Servers
}

// SetupClickHouse configures a client to use for testing. A random database is
// created for each test and dropped when the test ends.
func SetupClickHouse(t *testing.T, r *reporter.Reporter, cluster bool) *Component {
	t.Helper()
	config := DefaultConfiguration()
	if !cluster {
		config.Servers = []string{
			helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse:9000", "127.0.0.1:9000"}),
		}
	} else {
		// Each server of the test cluster, either from another container or
		// from the host. See docker-compose-dev.yml.
		servers := []string{}
		for _, candidates := range [][]string{
			{"clickhouse-1:9000", "127.0.0.1:9001"},
			{"clickhouse-3:9000", "127.0.0.1:9003"},
		} {
			servers = append(servers,
				helpers.CheckExternalService(t, "ClickHouse cluster", candidates))
		}
		config.Servers = servers
		config.Cluster = "akvorado"
	}
	config.DialTimeout = 5 * time.Second
	config.MaxOpenConns = 20
	config.Database = setupClickHouseDatabase(t, config.Servers, config.Cluster)

	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
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
