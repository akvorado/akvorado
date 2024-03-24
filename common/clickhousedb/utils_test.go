// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"testing"

	"akvorado/common/helpers"
)

func TestTransformQueryOnCluster(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
	}{
		{"SYSTEM RELOAD DICTIONARIES", "SYSTEM RELOAD DICTIONARIES ON CLUSTER akvorado"},
		{"system reload dictionaries", "system reload dictionaries ON CLUSTER akvorado"},
		{"  system reload  dictionaries ", "system reload dictionaries ON CLUSTER akvorado"},
		{"DROP DATABASE IF EXISTS 02028_db", "DROP DATABASE IF EXISTS 02028_db ON CLUSTER akvorado"},
		{
			"CREATE TABLE test_01148_atomic.rmt2 (n int, PRIMARY KEY n) ENGINE=ReplicatedMergeTree",
			"CREATE TABLE test_01148_atomic.rmt2 ON CLUSTER akvorado (n int, PRIMARY KEY n) ENGINE=ReplicatedMergeTree",
		},
		{
			"DROP TABLE IF EXISTS test_repl NO DELAY",
			"DROP TABLE IF EXISTS test_repl ON CLUSTER akvorado NO DELAY",
		},
		{
			"ALTER TABLE 02577_keepermap_delete_update UPDATE value2 = value2 * 10 + 2 WHERE value2 < 100",
			"ALTER TABLE 02577_keepermap_delete_update ON CLUSTER akvorado UPDATE value2 = value2 * 10 + 2 WHERE value2 < 100",
		},
		{"ATTACH DICTIONARY db_01018.dict1", "ATTACH DICTIONARY db_01018.dict1 ON CLUSTER akvorado"},
		{
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
			`CREATE DICTIONARY default.asns ON CLUSTER akvorado ( asn UInt32 INJECTIVE, name String ) PRIMARY KEY asn SOURCE(HTTP(URL 'http://akvorado-orchestrator:8080/api/v0/orchestrator/clickhouse/asns.csv' FORMAT 'CSVWithNames')) LIFETIME(MIN 0 MAX 3600) LAYOUT(HASHED()) SETTINGS(format_csv_allow_single_quotes = 0)`,
		},
		{
			`
CREATE TABLE queue (
    timestamp UInt64,
    level String,
    message String
  ) ENGINE = Kafka('localhost:9092', 'topic', 'group1', 'JSONEachRow')
`,
			`CREATE TABLE queue ON CLUSTER akvorado ( timestamp UInt64, level String, message String ) ENGINE = Kafka('localhost:9092', 'topic', 'group1', 'JSONEachRow')`,
		},
		{
			`
CREATE MATERIALIZED VIEW consumer TO daily
AS SELECT toDate(toDateTime(timestamp)) AS day, level, count() as total
FROM queue GROUP BY day, level
`,
			`CREATE MATERIALIZED VIEW consumer ON CLUSTER akvorado TO daily AS SELECT toDate(toDateTime(timestamp)) AS day, level, count() as total FROM queue GROUP BY day, level`,
		},
		// Not modified
		{"SELECT 1", "SELECT 1"},
	}
	for _, tc := range cases {
		got := TransformQueryOnCluster(tc.Input, "akvorado")
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("TransformQueryOnCluster(%q) (-got +want):\n%s", tc.Input, diff)
		}
	}
}
