package clickhouse

import (
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/daemon"
	"akvorado/helpers"
	"akvorado/http"
	"akvorado/kafka"
	"akvorado/reporter"
)

func TestRealClickhouse(t *testing.T) {
	chServer := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse", "localhost"}, "9000")

	configuration := DefaultConfiguration
	configuration.Servers = []string{chServer}
	r := reporter.NewMock(t)
	kafka, _ := kafka.NewMock(t, r, kafka.DefaultConfiguration)
	ch, err := New(r, configuration, Dependencies{
		Daemon: daemon.NewMock(t),
		Kafka:  kafka,
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

	// Check with the Clickhouse client we have our tables
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
		got = append(got, table)
	}
	expected := []string{
		"asns",
		"flows",
		"flows_0_raw",
		"flows_0_raw_consumer",
		"protocols",
		"samplers",
		"samplers_inif",
		"samplers_outif",
		"schema_migrations",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("SHOW TABLES (-got, +want):\n%s", diff)
	}
}
