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
	"akvorado/orchestrator/geoip"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type tableWithSchema struct {
	Table  string
	Schema string
}

const dumpAllTablesQuery = `
SELECT table, create_table_query
FROM system.tables
WHERE database=currentDatabase() AND table NOT LIKE '.%'
ORDER BY indexOf(['Dictionary'], engine) DESC, indexOf(['Distributed', 'MaterializedView'], engine) ASC
`

func dumpAllTables(t *testing.T, ch *clickhousedb.Component, schemaComponent *schema.Component) []tableWithSchema {
	// TODO: find the right ordering, this one does not totally work
	rows, err := ch.Query(context.Background(), dumpAllTablesQuery)
	if err != nil {
		t.Fatalf("Query() error:\n%+v", err)
	}
	schemas := []tableWithSchema{}
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&table, &schema); err != nil {
			t.Fatalf("Scan() error:\n%+v", err)
		}
		if !isOldTable(schemaComponent, table) {
			schemas = append(schemas, tableWithSchema{table, schema})
		}
	}
	return schemas
}

func dropAllTables(t *testing.T, ch *clickhousedb.Component) {
	t.Logf("(%s) Drop database default", time.Now())
	for _, sql := range []string{"DROP DATABASE IF EXISTS default SYNC", "CREATE DATABASE IF NOT EXISTS default"} {
		if err := ch.ExecOnCluster(context.Background(), sql); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", sql, err)
		}
	}
}

func loadTables(t *testing.T, ch *clickhousedb.Component, sch *schema.Component, schemas []tableWithSchema) {
	ctx := clickhouse.Context(context.Background(), clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	for _, tws := range schemas {
		if isOldTable(sch, tws.Table) {
			continue
		}
		t.Logf("Load table %s", tws.Table)
		if err := ch.ExecOnCluster(ctx, tws.Schema); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", tws.Schema, err)
		}
	}
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
			Table:  record[0],
			Schema: record[1],
		})
	}
	dropAllTables(t, ch)
	t.Logf("(%s) Load all tables from dump %s", time.Now(), filename)
	loadTables(t, ch, sch, schemas)
	t.Logf("(%s) Loaded all tables from dump %s", time.Now(), filename)
}

func isOldTable(schema *schema.Component, table string) bool {
	if strings.Contains(table, schema.ProtobufMessageHash()) {
		return false
	}
	if table == "flows_raw_errors" {
		return false
	}
	if strings.HasSuffix(table, "_raw") || strings.HasSuffix(table, "_raw_consumer") || strings.HasSuffix(table, "_raw_errors") {
		return true
	}
	return false
}

// startTestComponent starts a test component and wait for migrations to be done
func startTestComponent(t *testing.T, r *reporter.Reporter, chComponent *clickhousedb.Component, sch *schema.Component) *Component {
	t.Helper()
	if sch == nil {
		sch = schema.NewMock(t)
	}
	configuration := DefaultConfiguration()
	configuration.OrchestratorURL = "http://127.0.0.1:0"
	configuration.Kafka.Configuration = kafka.DefaultConfiguration()
	// This is a bit hacky, in real setup, the same configuration block is
	// used for both clickhousedb.Component and clickhouse.Component.
	configuration.Cluster = chComponent.ClusterName()
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
	return ch
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
	clickHouseComponent := clickhousedb.SetupClickHouse(t, r, false)
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

func testMigrationFromPreviousStates(t *testing.T, cluster bool) {
	var lastRun []tableWithSchema
	var lastSteps int
	files, err := os.ReadDir("testdata/states")
	if err != nil {
		t.Fatalf("ReadDir(%q) error:\n%+v", "testdata/states", err)
	}

	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r, cluster)

	for _, f := range files {
		if !cluster && strings.Contains(f.Name(), "cluster") {
			continue
		}
		if cluster && !strings.Contains(f.Name(), "cluster") {
			continue
		}
		if ok := t.Run(fmt.Sprintf("from %s", f.Name()), func(t *testing.T) {
			loadAllTables(t, chComponent, schema.NewMock(t), path.Join("testdata/states", f.Name()))
			r := reporter.NewMock(t)
			ch := startTestComponent(t, r, chComponent, nil)

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
				if !isOldTable(ch.d.Schema, table) {
					got = append(got, table)
				}
			}
			expected := []string{
				schema.DictionaryASNs,
				"exporters",
				"exporters_consumer",
				// No exporters_local, because exporters is always local
				"flows",
				"flows_1h0m0s",
				"flows_1h0m0s_consumer",
				"flows_1h0m0s_local",
				"flows_1m0s",
				"flows_1m0s_consumer",
				"flows_1m0s_local",
				"flows_5m0s",
				"flows_5m0s_consumer",
				"flows_5m0s_local",
				fmt.Sprintf("flows_%s_raw", hash),
				fmt.Sprintf("flows_%s_raw_consumer", hash),
				"flows_local",
				"flows_raw_errors",
				"flows_raw_errors_consumer",
				"flows_raw_errors_local",
				schema.DictionaryICMP,
				schema.DictionaryNetworks,
				schema.DictionaryProtocols,
			}
			if !cluster {
				filteredExpected := []string{}
				for _, item := range expected {
					if !strings.HasSuffix(item, "_local") {
						filteredExpected = append(filteredExpected, item)
					}
				}
				expected = filteredExpected
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("SHOW TABLES (-got, +want):\n%s", diff)
			}

			currentRun := dumpAllTables(t, chComponent, ch.d.Schema)
			if lastRun != nil {
				// Update ORDER BY for flows table
				for _, table := range [][]tableWithSchema{lastRun, currentRun} {
					for idx := range table {
						if table[idx].Table == ch.localTable("flows") {
							table[idx].Schema = strings.Replace(
								table[idx].Schema,
								"ORDER BY (TimeReceived, ",
								"ORDER BY (toStartOfFiveMinutes(TimeReceived), ", 1)
						}
					}
				}
				if diff := helpers.Diff(lastRun, currentRun); diff != "" {
					t.Fatalf("Final state is different (-last, +current):\n%s", diff)
				}
			}
			gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
			lastRun = currentRun
			lastSteps, _ = strconv.Atoi(gotMetrics["applied_steps_total"])
			t.Logf("%d steps applied for this migration", lastSteps)
		}); !ok {
			return
		}
	}

	_ = t.Run("idempotency", func(t *testing.T) {
		r := reporter.NewMock(t)
		startTestComponent(t, r, chComponent, nil)

		// No migration should have been applied the last time
		gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
		expectedMetrics := map[string]string{`applied_steps_total`: "0"}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Fatalf("Metrics (-got, +want):\n%s", diff)
		}
	}) && t.Run("final state", func(t *testing.T) {
		if lastSteps != 0 {
			f, err := os.CreateTemp("", "clickhouse-dump-*.csv")
			if err != nil {
				t.Fatalf("CreateTemp() error:\n%+v", err)
			}
			defer f.Close()
			writer := csv.NewWriter(f)
			defer writer.Flush()
			allTables := dumpAllTables(t, chComponent, schema.NewMock(t))
			for _, item := range allTables {
				writer.Write([]string{item.Table, item.Schema})
			}
			t.Fatalf("Last step was not idempotent. Check %s for the current dump", f.Name())
		}
	})
}

func TestMigrationFromPreviousStates(t *testing.T) {
	_ = t.Run("no cluster", func(t *testing.T) {
		testMigrationFromPreviousStates(t, false)
	}) && t.Run("full schema", func(t *testing.T) {
		r := reporter.NewMock(t)
		chComponent := clickhousedb.SetupClickHouse(t, r, false)
		startTestComponent(t, r, chComponent, schema.NewMock(t).EnableAllColumns())

		// We need to have at least one migration
		gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
		if gotMetrics["applied_steps_total"] == "0" {
			t.Fatal("No migration applied when enabling all columns")
		}
	}) && t.Run("partial schema", func(t *testing.T) {
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
		chComponent := clickhousedb.SetupClickHouse(t, r, false)
		startTestComponent(t, r, chComponent, sch)

		// We need to have at least one migration
		gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
		if gotMetrics["applied_steps_total"] == "0" {
			t.Fatal("No migration applied when disabling some columns")
		}
	}) && t.Run("cluster", func(t *testing.T) {
		testMigrationFromPreviousStates(t, true)
	})
}

func TestCustomDictMigration(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r, false)
	dropAllTables(t, chComponent)
	startTestComponent(t, r, chComponent, nil)

	// We need to have at least one migration
	gotMetrics := r.GetMetrics("akvorado_orchestrator_clickhouse_migrations_", "applied_steps_total")
	if gotMetrics["applied_steps_total"] == "0" {
		t.Fatal("No migration applied when applying a fresh default schema")
	}

	// Now, create a custom dictionary on top
	_ = t.Run("add", func(t *testing.T) {
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
		ch := startTestComponent(t, r, chComponent, sch)

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
		var got string
		if err := dictCreate.Scan(&got); err != nil {
			t.Fatalf("Scan() error:\n%+v", err)
		}
		expected := `CREATE DICTIONARY default.custom_dict_test
(
    ` + "`SrcAddr`" + ` String,
    ` + "`csv_col_name`" + ` String DEFAULT 'None',
    ` + "`csv_col_default`" + ` String DEFAULT 'Hello World'
)
PRIMARY KEY SrcAddr
SOURCE(HTTP(URL 'http://127.0.0.1:0/api/v0/orchestrator/clickhouse/custom_dict_test.csv' FORMAT 'CSVWithNames'))
LIFETIME(MIN 0 MAX 3600)
LAYOUT(COMPLEX_KEY_HASHED())
SETTINGS(format_csv_allow_single_quotes = 0)`
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("Unexpected state:\n%s", diff)
		}
	}) && t.Run("remove", func(t *testing.T) {
		// Next test: with the custom dict removed again, the cols should still exist, but the consumer should be gone
		r := reporter.NewMock(t)
		sch, err := schema.New(schema.DefaultConfiguration())

		if err != nil {
			t.Fatalf("schema.New() error:\n%+v", err)
		}
		ch := startTestComponent(t, r, chComponent, sch)
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
		rowConsumer := ch.d.ClickHouse.QueryRow(context.Background(),
			`SHOW CREATE flows_LAABIGYMRYZPTGOYIIFZNYDEQM_raw_consumer`)
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
				t.Fatalf("Unexpected statement found in consumer:\n%s", s)
			}
		}
	})
}
