// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"testing"

	"akvorado/common/helpers"
)

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct {
		Pos      helpers.Pos
		Input    string
		Expected string
	}{
		{helpers.Mark(), "akvorado", "akvorado"},
		{helpers.Mark(), "simple_name", "simple_name"},
		{helpers.Mark(), "_leading", "_leading"},
		{helpers.Mark(), "my`cluster", "`my``cluster`"},
		{helpers.Mark(), "with spaces", "`with spaces`"},
		{helpers.Mark(), "with-dash", "`with-dash`"},
		{helpers.Mark(), "123start", "`123start`"},
		{helpers.Mark(), "", "``"},
	}
	for _, tc := range cases {
		got := QuoteIdentifier(tc.Input)
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("%sQuoteIdentifier(%q) (-got, +want):\n%s", tc.Pos, tc.Input, diff)
		}
	}
}

func TestTransformQueryOnCluster(t *testing.T) {
	cases := []struct {
		Pos      helpers.Pos
		Input    string
		Cluster  string
		Expected string
	}{
		{helpers.Mark(), "SYSTEM RELOAD DICTIONARIES", "akvorado", "SYSTEM RELOAD DICTIONARIES ON CLUSTER akvorado"},
		{helpers.Mark(), "system reload dictionaries", "akvorado", "system reload dictionaries ON CLUSTER akvorado"},
		{helpers.Mark(), "  system reload  dictionaries ", "akvorado", "system reload dictionaries ON CLUSTER akvorado"},
		{helpers.Mark(), "DROP DATABASE IF EXISTS 02028_db", "akvorado", "DROP DATABASE IF EXISTS 02028_db ON CLUSTER akvorado"},
		{
			helpers.Mark(),
			"CREATE TABLE test_01148_atomic.rmt2 (n int, PRIMARY KEY n) ENGINE=ReplicatedMergeTree",
			"akvorado",
			"CREATE TABLE test_01148_atomic.rmt2 ON CLUSTER akvorado (n int, PRIMARY KEY n) ENGINE=ReplicatedMergeTree",
		},
		{
			helpers.Mark(),
			"DROP TABLE IF EXISTS test_repl NO DELAY",
			"akvorado",
			"DROP TABLE IF EXISTS test_repl ON CLUSTER akvorado NO DELAY",
		},
		{
			helpers.Mark(),
			"ALTER TABLE 02577_keepermap_delete_update UPDATE value2 = value2 * 10 + 2 WHERE value2 < 100",
			"akvorado",
			"ALTER TABLE 02577_keepermap_delete_update ON CLUSTER akvorado UPDATE value2 = value2 * 10 + 2 WHERE value2 < 100",
		},
		{
			helpers.Mark(), "ATTACH DICTIONARY db_01018.dict1",
			"akvorado",
			"ATTACH DICTIONARY db_01018.dict1 ON CLUSTER akvorado",
		},
		{
			helpers.Mark(),
			`CREATE DICTIONARY default.asns
(
    asn UInt32 INJECTIVE,
    name String
)
PRIMARY KEY asn
SOURCE(HTTP(URL 'http://akvorado-orchestrator:8080/api/v0/orchestrator/clickhouse/asns.csv' FORMAT 'CSVWithNames'))
LIFETIME(MIN 0 MAX 3600)
LAYOUT(HASHED())
SETTINGS(format_csv_allow_single_quotes = 0)`,
			"akvorado",
			`CREATE DICTIONARY default.asns ON CLUSTER akvorado ( asn UInt32 INJECTIVE, name String ) PRIMARY KEY asn SOURCE(HTTP(URL 'http://akvorado-orchestrator:8080/api/v0/orchestrator/clickhouse/asns.csv' FORMAT 'CSVWithNames')) LIFETIME(MIN 0 MAX 3600) LAYOUT(HASHED()) SETTINGS(format_csv_allow_single_quotes = 0)`,
		},
		{
			helpers.Mark(),
			`
CREATE TABLE queue (
    timestamp UInt64,
    level String,
    message String
  ) ENGINE = Kafka('localhost:9092', 'topic', 'group1', 'JSONEachRow')
`,
			"akvorado",
			`CREATE TABLE queue ON CLUSTER akvorado ( timestamp UInt64, level String, message String ) ENGINE = Kafka('localhost:9092', 'topic', 'group1', 'JSONEachRow')`,
		},
		{
			helpers.Mark(),
			`
CREATE MATERIALIZED VIEW consumer TO daily
AS SELECT toDate(toDateTime(timestamp)) AS day, level, count() as total
FROM queue GROUP BY day, level
`,
			"akvorado",
			`CREATE MATERIALIZED VIEW consumer ON CLUSTER akvorado TO daily AS SELECT toDate(toDateTime(timestamp)) AS day, level, count() as total FROM queue GROUP BY day, level`,
		},
		// Cluster name needing quoting
		{helpers.Mark(), "SYSTEM RELOAD DICTIONARIES", "my-cluster", "SYSTEM RELOAD DICTIONARIES ON CLUSTER `my-cluster`"},
		// Not modified
		{helpers.Mark(), "SELECT 1", "akvorado", "SELECT 1"},
	}
	for _, tc := range cases {
		got := TransformQueryOnCluster(tc.Input, tc.Cluster)
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("%sTransformQueryOnCluster() (-got +want):\n%s", tc.Pos, diff)
		}
	}
}
