// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/kafka"
	"akvorado/common/reporter"

	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
)

var ignoredTables = []string{
	"flows_1_raw",
	"flows_1_raw_consumer",
}

func dropAllTables(t *testing.T, ch *clickhousedb.Component) {
	// TODO: find the right order. length(table) ordering works good enough here.
	rows, err := ch.Query(context.Background(), `
SELECT engine, table
FROM system.tables
WHERE database=currentDatabase() AND table NOT LIKE '.%'
ORDER BY length(table) DESC`)
	if err != nil {
		t.Fatalf("Query() error:\n%+v", err)
	}
	for rows.Next() {
		var engine, table, sql string
		if err := rows.Scan(&engine, &table); err != nil {
			t.Fatalf("Scan() error:\n%+v", err)
		}
		t.Logf("(%s) Drop table %s", time.Now(), table)
		switch engine {
		case "Dictionary":
			sql = "DROP DICTIONARY %s SYNC"
		default:
			sql = "DROP TABLE %s SYNC"
		}
		if err := ch.Exec(context.Background(), fmt.Sprintf(sql, table)); err != nil {
			t.Fatalf("Exec() error:\n%+v", err)
		}
	}
}

func dumpAllTables(t *testing.T, ch *clickhousedb.Component) map[string]string {
	// TODO: find the right ordering, this one does not totally work
	rows, err := ch.Query(context.Background(), `
SELECT table, create_table_query
FROM system.tables
WHERE database=currentDatabase() AND table NOT LIKE '.%'
ORDER BY length(table) ASC`)
	if err != nil {
		t.Fatalf("Query() error:\n%+v", err)
	}
	schemas := map[string]string{}
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&table, &schema); err != nil {
			t.Fatalf("Scan() error:\n%+v", err)
		}
		schemas[table] = schema
	}
	return schemas
}

type tableWithSchema struct {
	table  string
	schema string
}

func loadTables(t *testing.T, ch *clickhousedb.Component, schemas []tableWithSchema) {
outer:
	for _, tws := range schemas {
		for _, ignored := range ignoredTables {
			if ignored == tws.table {
				continue outer
			}
		}
		t.Logf("Load table %s", tws.table)
		if err := ch.Exec(context.Background(), tws.schema); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", tws.schema, err)
		}
	}
}

// loadAllTables load tables from a CSV file. Use `format CSV` with
// query from dumpAllTables.
func loadAllTables(t *testing.T, ch *clickhousedb.Component, filename string) {
	input, err := os.Open(filename)
	if err != nil {
		t.Fatalf("Open(%q) error:\n%+v", filename, err)
	}
	defer input.Close()
	schemas := []tableWithSchema{}
	r := csv.NewReader(input)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read(%q) error:\n%+v", filename, err)
		}
		if len(record) == 0 {
			continue
		}
		schemas = append(schemas, tableWithSchema{
			table:  record[0],
			schema: record[1],
		})
	}
	dropAllTables(t, ch)
	t.Logf("(%s) Load all tables from dump %s", time.Now(), filename)
	loadTables(t, ch, schemas)
	t.Logf("(%s) Loaded all tables from dump %s", time.Now(), filename)
}

func waitMigrations(t *testing.T, ch *Component) {
	t.Helper()
	select {
	case <-ch.migrationsDone:
	case <-ch.migrationsOnce:
		select {
		case <-ch.migrationsDone:
		default:
			t.Fatalf("Migrations failed")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("Migrations not finished")
	}
}

func TestGetHTTPBaseURL(t *testing.T) {
	r := reporter.NewMock(t)
	http := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	rawURL, err := c.getHTTPBaseURL("8.8.8.8:9000")
	if err != nil {
		t.Fatalf("getHTTPBaseURL() error:\n%+v", err)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse(%q) error:\n%+v", rawURL, err)
	}
	expectedURL := &url.URL{
		Scheme: "http",
		Host:   http.LocalAddr().String(),
	}
	parsedURL.Host = parsedURL.Host[strings.LastIndex(parsedURL.Host, ":"):]
	expectedURL.Host = expectedURL.Host[strings.LastIndex(expectedURL.Host, ":"):]
	// We can't really know our IP
	if diff := helpers.Diff(parsedURL, expectedURL); diff != "" {
		t.Fatalf("getHTTPBaseURL() (-got, +want):\n%s", diff)
	}
}

func TestMigration(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r)

	var lastRun map[string]string
	files, err := ioutil.ReadDir("testdata/states")
	if err != nil {
		t.Fatalf("ReadDir(%q) error:\n%+v", "testdata/states", err)
	}
	for _, f := range files {
		t.Run(f.Name(), func(t *testing.T) {
			loadAllTables(t, chComponent, path.Join("testdata/states", f.Name()))
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://something"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       http.NewMock(t, r),
				ClickHouse: chComponent,
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// Check with the ClickHouse client we have our tables
			rows, err := chComponent.Query(context.Background(), `
SELECT table
FROM system.tables
WHERE database=currentDatabase() AND table NOT LIKE '.%'`)
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
				"exporters",
				"flows",
				"flows_1h0m0s",
				"flows_1h0m0s_consumer",
				"flows_1m0s",
				"flows_1m0s_consumer",
				"flows_4_raw",
				"flows_4_raw_consumer",
				"flows_4_raw_errors",
				"flows_5m0s",
				"flows_5m0s_consumer",
				"networks",
				"protocols",
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("SHOW TABLES (-got, +want):\n%s", diff)
			}

			currentRun := dumpAllTables(t, chComponent)
			if lastRun != nil {
				if diff := helpers.Diff(lastRun, currentRun); diff != "" {
					t.Fatalf("Final state is different (-last, +current):\n%s", diff)
				}
			}
			lastRun = currentRun
		})
		if t.Failed() {
			row := chComponent.QueryRow(context.Background(), `
SELECT query
FROM system.query_log
WHERE client_name = $1
ORDER BY event_time_microseconds DESC
LIMIT 1`, proto.ClientName)
			var lastQuery string
			if err := row.Scan(&lastQuery); err == nil {
				t.Logf("last ClickHouse query: %s", lastQuery)
			}
			break
		}
	}

	if !t.Failed() {
		// One more time
		t.Run("idempotency", func(t *testing.T) {
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://something"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       http.NewMock(t, r),
				ClickHouse: chComponent,
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// No migration should have been applied the last time
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_",
				"applied_steps")
			expectedMetrics := map[string]string{
				`applied_steps`: "0",
			}
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			}
		})
	}
}
