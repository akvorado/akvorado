package clickhouse

import (
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestRealClickHouse(t *testing.T) {
	chServer := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse", "localhost"}, "9000")

	configuration := DefaultConfiguration
	configuration.Servers = []string{chServer}
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
	defer func() {
		if err := ch.Stop(); err != nil {
			t.Fatalf("Stop() error:\n%+v", err)
		}
	}()
	select {
	case <-ch.migrationsDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("Migrations not done")
	}

	// Check with the ClickHouse client we have our tables
	conn := clickhouse.OpenDB(&clickhouse.Options{
		Addr: []string{chServer},
		Auth: clickhouse.Auth{
			Database: ch.config.Database,
			Username: ch.config.Username,
			Password: ch.config.Password,
		},
		DialTimeout: 100 * time.Millisecond,
	})
	if err := conn.Ping(); err != nil {
		t.Fatalf("Ping() error:\n%+v", err)
	}
	rows, err := conn.Query("SHOW TABLES")
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
}
