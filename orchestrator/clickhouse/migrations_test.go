// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/kafka"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/orchestrator/clickhouse/geoip"

	"github.com/ClickHouse/clickhouse-go/v2"
)

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

const dumpAllTablesQuery = `
SELECT table, create_table_query
FROM system.tables
WHERE database=currentDatabase() AND table NOT LIKE '.%'
ORDER BY length(table) ASC`

func dumpAllTables(t *testing.T, ch *clickhousedb.Component, schemaComponent *schema.Component) map[string]string {
	// TODO: find the right ordering, this one does not totally work
	rows, err := ch.Query(context.Background(), dumpAllTablesQuery)
	if err != nil {
		t.Fatalf("Query() error:\n%+v", err)
	}
	schemas := map[string]string{}
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&table, &schema); err != nil {
			t.Fatalf("Scan() error:\n%+v", err)
		}
		if !oldTable(schemaComponent, table) {
			schemas[table] = schema
		}
	}
	return schemas
}

type tableWithSchema struct {
	table  string
	schema string
}

func loadTables(t *testing.T, ch *clickhousedb.Component, sch *schema.Component, schemas []tableWithSchema) {
	ctx := clickhouse.Context(context.Background(), clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	for _, tws := range schemas {
		if oldTable(sch, tws.table) {
			continue
		}
		t.Logf("Load table %s", tws.table)
		if err := ch.Exec(ctx, tws.schema); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", tws.schema, err)
		}
	}
}

func oldTable(schema *schema.Component, table string) bool {
	if strings.Contains(table, schema.ProtobufMessageHash()) {
		return false
	}
	if strings.HasSuffix(table, "_raw") || strings.HasSuffix(table, "_raw_consumer") || strings.HasSuffix(table, "_raw_errors") {
		return true
	}
	return false
}

// loadAllTables load tables from a CSV file. Use `format CSV` with
// query from dumpAllTables.
func loadAllTables(t *testing.T, ch *clickhousedb.Component, sch *schema.Component, filename string) {
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
	loadTables(t, ch, sch, schemas)
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
	t.Log("Migrations done")
}

func TestGetHTTPBaseURL(t *testing.T) {
	r := reporter.NewMock(t)
	clickHouseComponent := clickhousedb.SetupClickHouse(t, r)
	http := httpserver.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon:     daemon.NewMock(t),
		HTTP:       http,
		Schema:     schema.NewMock(t),
		GeoIP:      geoip.NewMock(t, r, true),
		ClickHouse: clickHouseComponent,
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
	if err := chComponent.Exec(context.Background(), "DROP TABLE IF EXISTS system.metric_log"); err != nil {
		t.Fatalf("Exec() error:\n%+v", err)
	}

	var lastRun map[string]string
	var lastSteps int
	files, err := os.ReadDir("testdata/states")
	if err != nil {
		t.Fatalf("ReadDir(%q) error:\n%+v", "testdata/states", err)
	}
	for _, f := range files {
		t.Run(f.Name(), func(t *testing.T) {
			loadAllTables(t, chComponent, schema.NewMock(t), path.Join("testdata/states", f.Name()))
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     schema.NewMock(t),
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
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
			hash := ch.d.Schema.ProtobufMessageHash()
			got := []string{}
			for rows.Next() {
				var table string
				if err := rows.Scan(&table); err != nil {
					t.Fatalf("Scan() error:\n%+v", err)
				}
				if !oldTable(ch.d.Schema, table) {
					got = append(got, table)
				}
			}
			expected := []string{
				schema.DictionaryASNs,
				"exporters",
				"flows",
				"flows_1h0m0s",
				"flows_1h0m0s_consumer",
				"flows_1m0s",
				"flows_1m0s_consumer",
				"flows_5m0s",
				"flows_5m0s_consumer",
				fmt.Sprintf("flows_%s_raw", hash),
				fmt.Sprintf("flows_%s_raw_consumer", hash),
				fmt.Sprintf("flows_%s_raw_errors", hash),
				schema.DictionaryICMP,
				schema.DictionaryNetworks,
				schema.DictionaryProtocols,
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("SHOW TABLES (-got, +want):\n%s", diff)
			}

			currentRun := dumpAllTables(t, chComponent, ch.d.Schema)
			if lastRun != nil {
				if diff := helpers.Diff(lastRun, currentRun); diff != "" {
					t.Fatalf("Final state is different (-last, +current):\n%s", diff)
				}
			}
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			lastRun = currentRun
			lastSteps, _ = strconv.Atoi(gotMetrics["applied_steps_total"])
			t.Logf("%d steps applied for this migration", lastSteps)
		})
		if t.Failed() {
			row := chComponent.QueryRow(context.Background(), `
SELECT query, exception
FROM system.query_log
WHERE client_name LIKE 'akvorado/%'
AND query NOT LIKE '%ORDER BY event_time_microseconds%'
ORDER BY event_time_microseconds DESC
LIMIT 1`)
			var lastQuery, exception string
			if err := row.Scan(&lastQuery, &exception); err == nil {
				t.Logf("last ClickHouse query: %s", lastQuery)
				if exception != "" {
					t.Logf("last ClickHouse error: %s", exception)
				}
			}
			break
		}
	}

	if !t.Failed() {
		// One more time
		t.Run("idempotency", func(t *testing.T) {
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     schema.NewMock(t),
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// No migration should have been applied the last time
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			expectedMetrics := map[string]string{`applied_steps_total`: "0"}
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			}
		})
	}

	if !t.Failed() {
		t.Run("final state", func(t *testing.T) {
			if lastSteps != 0 {
				t.Fatalf("Last step was not idempotent. Record a new one with:\n%s FORMAT CSV", dumpAllTablesQuery)
			}
		})
	}

	// Also try with a full schema
	if !t.Failed() {
		t.Run("full schema", func(t *testing.T) {
			r := reporter.NewMock(t)
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     schema.NewMock(t).EnableAllColumns(),
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// We need to have at least one migration
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			if gotMetrics["applied_steps_total"] == "0" {
				t.Fatal("No migration applied when enabling all columns")
			}
		})
	}

	// And with a partial one
	if !t.Failed() {
		t.Run("partial schema", func(t *testing.T) {
			r := reporter.NewMock(t)
			schConfig := schema.DefaultConfiguration()
			schConfig.Disabled = []schema.ColumnKey{
				schema.ColumnDst1stAS, schema.ColumnDst2ndAS, schema.ColumnDst3rdAS,
				schema.ColumnDstASPath,
				schema.ColumnDstCommunities,
				schema.ColumnDstLargeCommunities,
				schema.ColumnDstLargeCommunitiesASN,
				schema.ColumnDstLargeCommunitiesLocalData1,
				schema.ColumnDstLargeCommunitiesLocalData2,
			}
			sch, err := schema.New(schConfig)
			if err != nil {
				t.Fatalf("schema.New() error:\n%+v", err)
			}
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     sch,
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// We need to have at least one migration
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			if gotMetrics["applied_steps_total"] == "0" {
				t.Fatal("No migration applied when disabling some columns")
			}
		})
	}

	// Convert a column from alias to materialize
	if !t.Failed() {
		t.Run("materialize alias", func(t *testing.T) {
			r := reporter.NewMock(t)
			schConfig := schema.DefaultConfiguration()
			schConfig.Materialize = []schema.ColumnKey{
				schema.ColumnSrcNetPrefix,
			}
			sch, err := schema.New(schConfig)
			if err != nil {
				t.Fatalf("schema.New() error:\n%+v", err)
			}
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     sch,
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// We need to have at least one migration
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			if gotMetrics["applied_steps_total"] == "0" {
				t.Fatal("No migration applied when disabling some columns")
			}

			// We need SrcNetPrefix materialized and DstNetPrefix an alias
			row := ch.d.ClickHouse.QueryRow(context.Background(), `
SELECT toString(groupArray(tuple(name, default_kind)))
FROM system.columns
WHERE table = $1
AND database = $2
AND name LIKE $3`, "flows", ch.config.Database, "%NetPrefix")
			var existing string
			if err := row.Scan(&existing); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			if diff := helpers.Diff(existing, "[('SrcNetPrefix',''),('DstNetPrefix','ALIAS')]"); diff != "" {
				t.Fatalf("Unexpected state:\n%s", diff)
			}
		})
	}
}

func TestCustomDictMigration(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r)
	if err := chComponent.Exec(context.Background(), "DROP TABLE IF EXISTS system.metric_log"); err != nil {
		t.Fatalf("Exec() error:\n%+v", err)
	}
	// start clean
	dropAllTables(t, chComponent)
	// First, setup a default configuration
	t.Run("default schema", func(t *testing.T) {
		r := reporter.NewMock(t)
		sch, err := schema.New(schema.DefaultConfiguration())
		if err != nil {
			t.Fatalf("schema.New() error:\n%+v", err)
		}
		configuration := DefaultConfiguration()
		configuration.OrchestratorURL = "http://127.0.0.1:0"
		configuration.Kafka.Configuration = kafka.DefaultConfiguration()
		ch, err := New(r, configuration, Dependencies{
			Daemon:     daemon.NewMock(t),
			HTTP:       httpserver.NewMock(t, r),
			Schema:     sch,
			ClickHouse: chComponent,
			GeoIP:      geoip.NewMock(t, r, true),
		})
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}
		helpers.StartStop(t, ch)
		waitMigrations(t, ch)

		// We need to have at least one migration
		gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
		if gotMetrics["applied_steps_total"] == "0" {
			t.Fatal("No migration applied when applying a fresh default schema")
		}
	})
	// Now, create a custom dictionary on top
	if !t.Failed() {
		t.Run("custom dictionary", func(t *testing.T) {
			r := reporter.NewMock(t)
			schConfig := schema.DefaultConfiguration()
			schConfig.CustomDictionaries = make(map[string]schema.CustomDict)
			schConfig.CustomDictionaries["test"] = schema.CustomDict{
				Keys: []schema.CustomDictKey{
					{Name: "SrcAddr", Type: "String"},
				},
				Attributes: []schema.CustomDictAttribute{
					{Name: "csv_col_name", Type: "String", Label: "DimensionAttribute"},
					{Name: "csv_col_default", Type: "String", Label: "DefaultDimensionAttribute", Default: "Hello World"},
				},
				Source:     "test.csv",
				Dimensions: []string{"SrcAddr", "DstAddr"},
				Layout:     "complex_key_hashed",
			}
			sch, err := schema.New(schConfig)

			if err != nil {
				t.Fatalf("schema.New() error:\n%+v", err)
			}
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     sch,
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// We need to have at least one migration
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			if gotMetrics["applied_steps_total"] == "0" {
				t.Fatal("No migration applied when enabling a custom dictionary")
			}

			// Check if the rows were created in the main flows table
			row := ch.d.ClickHouse.QueryRow(context.Background(), `
	SELECT toString(groupArray(tuple(name, type, default_expression)))
	FROM system.columns
	WHERE table = $1
	AND database = $2
	AND name LIKE $3`, "flows", ch.config.Database, "%DimensionAttribute")
			var existing string
			if err := row.Scan(&existing); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			if diff := helpers.Diff(existing,
				"[('SrcAddrDimensionAttribute','LowCardinality(String)',''),('SrcAddrDefaultDimensionAttribute','LowCardinality(String)',''),('DstAddrDimensionAttribute','LowCardinality(String)',''),('DstAddrDefaultDimensionAttribute','LowCardinality(String)','')]"); diff != "" {
				t.Fatalf("Unexpected state:\n%s", diff)
			}

			// Check if the rows were created in the consumer flows table
			rowConsumer := ch.d.ClickHouse.QueryRow(context.Background(), `
		SHOW CREATE flows_LAABIGYMRYZPTGOYIIFZNYDEQM_raw_consumer`)
			var existingConsumer string
			if err := rowConsumer.Scan(&existingConsumer); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			// Check if the definitions are part of the consumer
			expectedStatements := []string{
				"dictGet('default.custom_dict_test', 'csv_col_name', DstAddr) AS DstAddrDimensionAttribute",
				"dictGet('default.custom_dict_test', 'csv_col_name', SrcAddr) AS SrcAddrDimensionAttribute",
				"dictGet('default.custom_dict_test', 'csv_col_default', SrcAddr) AS SrcAddrDefaultDimensionAttribute",
				"dictGet('default.custom_dict_test', 'csv_col_default', DstAddr) AS DstAddrDefaultDimensionAttribute",
			}
			for _, s := range expectedStatements {
				if !strings.Contains(existingConsumer, s) {
					t.Fatalf("Missing statement in consumer:\n%s", s)
				}
			}

			// Check if the dictionary was created
			dictCreate := ch.d.ClickHouse.QueryRow(context.Background(), `
		SHOW CREATE custom_dict_test`)
			var dictCreateString string
			if err := dictCreate.Scan(&dictCreateString); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			if diff := helpers.Diff(dictCreateString,
				"CREATE DICTIONARY default.custom_dict_test\n(\n    `SrcAddr` String,\n    `csv_col_name` String DEFAULT 'None',\n    `csv_col_default` String DEFAULT 'Hello World'\n)\nPRIMARY KEY SrcAddr\nSOURCE(HTTP(URL 'http://127.0.0.1:0/api/v0/orchestrator/clickhouse/custom_dict_test.csv' FORMAT 'CSVWithNames'))\nLIFETIME(MIN 0 MAX 3600)\nLAYOUT(COMPLEX_KEY_HASHED())\nSETTINGS(format_csv_allow_single_quotes = 0)"); diff != "" {
				t.Fatalf("Unexpected state:\n%s", diff)
			}
		})
	}
	// Next test: with the custom dict removed again, the cols should still exist, but the consumer should be gone
	if !t.Failed() {
		t.Run("remove custom dictionary", func(t *testing.T) {
			r := reporter.NewMock(t)
			sch, err := schema.New(schema.DefaultConfiguration())

			if err != nil {
				t.Fatalf("schema.New() error:\n%+v", err)
			}
			configuration := DefaultConfiguration()
			configuration.OrchestratorURL = "http://127.0.0.1:0"
			configuration.Kafka.Configuration = kafka.DefaultConfiguration()
			ch, err := New(r, configuration, Dependencies{
				Daemon:     daemon.NewMock(t),
				HTTP:       httpserver.NewMock(t, r),
				Schema:     sch,
				ClickHouse: chComponent,
				GeoIP:      geoip.NewMock(t, r, true),
			})
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
			helpers.StartStop(t, ch)
			waitMigrations(t, ch)

			// We need to have at least one migration
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			if gotMetrics["applied_steps_total"] == "0" {
				t.Fatal("No migration applied when disabling the custom dict")
			}

			// Check if the rows were created in the main flows table
			row := ch.d.ClickHouse.QueryRow(context.Background(), `
	SELECT toString(groupArray(tuple(name, type, default_expression)))
	FROM system.columns
	WHERE table = $1
	AND database = $2
	AND name LIKE $3`, "flows", ch.config.Database, "%DimensionAttribute")
			var existing string
			if err := row.Scan(&existing); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			if diff := helpers.Diff(existing,
				"[('SrcAddrDimensionAttribute','LowCardinality(String)',''),('SrcAddrDefaultDimensionAttribute','LowCardinality(String)',''),('DstAddrDimensionAttribute','LowCardinality(String)',''),('DstAddrDefaultDimensionAttribute','LowCardinality(String)','')]"); diff != "" {
				t.Fatalf("Unexpected state:\n%s", diff)
			}

			// Check if the rows were removed in the consumer flows table
			rowConsumer := ch.d.ClickHouse.QueryRow(context.Background(), `
		SHOW CREATE flows_LAABIGYMRYZPTGOYIIFZNYDEQM_raw_consumer`)
			var existingConsumer string
			if err := rowConsumer.Scan(&existingConsumer); err != nil {
				t.Fatalf("Scan() error:\n%+v", err)
			}
			// Check if the definitions are missing in the consumer
			expectedStatements := []string{
				"dictGet('default.custom_dict_test', 'csv_col_name', DstAddr) AS DstAddrDimensionAttribute",
				"dictGet('default.custom_dict_test', 'csv_col_name', SrcAddr) AS SrcAddrDimensionAttribute",
				"dictGet('default.custom_dict_test', 'csv_col_default', SrcAddr) AS SrcAddrDefaultDimensionAttribute",
				"dictGet('default.custom_dict_test', 'csv_col_default', DstAddr) AS DstAddrDefaultDimensionAttribute",
			}
			for _, s := range expectedStatements {
				if strings.Contains(existingConsumer, s) {
					t.Fatalf("Unexpected Statement found in consumer:\n%s", s)
				}
			}
		})
	}
}
