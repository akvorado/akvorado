package clickhouse

import (
	"context"
	"strings"
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestRealClickHouse(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r)

	t.Run("first time", func(t *testing.T) {
		configuration := DefaultConfiguration()
		ch, err := New(r, configuration, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       http.NewMock(t, r),
			ClickHouse: chComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, ch)
		select {
		case <-ch.migrationsDone:
		case <-time.After(3 * time.Second):
			t.Fatalf("Migrations not done")
		}

		// Check with the ClickHouse client we have our tables
		rows, err := chComponent.Query(context.Background(), "SHOW TABLES")
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
	})

	t.Run("second time", func(t *testing.T) {
		configuration := DefaultConfiguration()
		ch, err := New(r, configuration, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       http.NewMock(t, r),
			ClickHouse: chComponent,
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, ch)
		select {
		case <-ch.migrationsDone:
		case <-time.After(3 * time.Second):
			t.Fatalf("Migrations not done")
		}
	})
}
