package clickhouse

import (
	"context"
	"strings"
	"testing"
	"time"

	"akvorado/common/clickhouse"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestRealClickHouse(t *testing.T) {
	conn, chServers := clickhouse.SetupClickHouse(t)

	configuration := DefaultConfiguration()
	configuration.Servers = chServers
	r := reporter.NewMock(t)
	ch, err := New(r, configuration, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := ch.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	select {
	case <-ch.migrationsDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("Migrations not done")
	}

	// Check with the ClickHouse client we have our tables
	rows, err := conn.Query(context.Background(), "SHOW TABLES")
	if err != nil {
		t.Fatalf("Query() error:\n%+v", err)
	}
	got := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("Scan() error:\n%+v", err)
		}
		if !strings.HasPrefix(table, ".") {
			got = append(got, table)
		}
	}
	expected := []string{
		"asns",
		"exporters",
		"flows",
		"flows_1_raw",
		"flows_1_raw_consumer",
		"protocols",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("SHOW TABLES (-got, +want):\n%s", diff)
	}
	if err := ch.Stop(); err != nil {
		t.Fatalf("Stop() error:\n%+v", err)
	}

	// Check we can run a second time
	ch, err = New(r, configuration, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := ch.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	select {
	case <-ch.migrationsDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("Migrations not done")
	}
	ch.Stop()
}
