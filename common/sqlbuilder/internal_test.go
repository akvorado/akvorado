// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"testing"

	"akvorado/common/helpers"
)

func TestCanonical(t *testing.T) {
	cases := []struct {
		Description string
		SQL         string
		Expected    string
	}{
		{
			Description: "quotes dropped where ClickHouse does not use them",
			SQL:         "CREATE TABLE `db`.`flows` (`SrcAddr` IPv6 CODEC(ZSTD(1))) ENGINE = MergeTree ORDER BY SrcAddr",
			Expected:    "CREATE TABLE db.flows (SrcAddr IPv6 CODEC(ZSTD(1))) ENGINE = MergeTree ORDER BY SrcAddr",
		}, {
			Description: "quotes kept where they are needed",
			SQL:         "CREATE TABLE `my db`.`flows-1` (`Src Addr` IPv6) ENGINE = Null",
			Expected:    "CREATE TABLE `my db`.`flows-1` (`Src Addr` IPv6) ENGINE = Null",
		}, {
			Description: "layout dropped",
			SQL:         "SELECT\n  SrcAS,\n  DstAS\nFROM   flows\nWHERE  SrcAS = 65000",
			Expected:    "SELECT SrcAS, DstAS FROM flows WHERE SrcAS = 65000",
		}, {
			Description: "engine name unquoted",
			SQL:         "CREATE TABLE flows (`SrcAS` UInt32) ENGINE = `Null`",
			Expected:    "CREATE TABLE flows (SrcAS UInt32) ENGINE = Null",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			got, ok := canonical(tc.SQL)
			if !ok {
				t.Fatalf("canonical(%q) could not parse", tc.SQL)
			}
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("canonical(%q) (-got, +want):\n%s", tc.SQL, diff)
			}
		})
	}
}

// TestCanonicalCompare checks what two statements are compared on. This is what
// tells if a table in ClickHouse still matches the one we would create.
func TestCanonicalCompare(t *testing.T) {
	cases := []struct {
		Description string
		First       string
		Second      string
		Expected    bool
	}{
		{
			Description: "same statement, other quoting",
			First:       "CREATE TABLE `db`.`flows` (`SrcAS` UInt32) ENGINE = Null",
			Second:      "CREATE TABLE db.flows (SrcAS UInt32) ENGINE = Null",
			Expected:    true,
		}, {
			Description: "same statement, clauses in the order ClickHouse writes them",
			First:       "CREATE TABLE flows (`SrcAS` UInt32) ENGINE = MergeTree PARTITION BY toYYYYMM(TimeReceived) ORDER BY SrcAS",
			Second:      "CREATE TABLE flows (`SrcAS` UInt32) ENGINE = MergeTree ORDER BY SrcAS PARTITION BY toYYYYMM(TimeReceived)",
			Expected:    true,
		}, {
			Description: "same dictionary, attributes quoted by ClickHouse only",
			First:       "CREATE DICTIONARY db.asns (`asn` UInt32 INJECTIVE, `name` String) PRIMARY KEY asn SOURCE(HTTP(URL 'http://x' FORMAT 'CSVWithNames')) LIFETIME(MIN 0 MAX 3600) LAYOUT(HASHED())",
			Second:      "CREATE DICTIONARY db.asns (asn UInt32 INJECTIVE, name String) PRIMARY KEY asn SOURCE(HTTP(URL 'http://x' FORMAT 'CSVWithNames')) LIFETIME(MIN 0 MAX 3600) LAYOUT(HASHED())",
			Expected:    true,
		}, {
			Description: "another column type",
			First:       "CREATE TABLE flows (`SrcAS` UInt32) ENGINE = Null",
			Second:      "CREATE TABLE flows (`SrcAS` UInt64) ENGINE = Null",
			Expected:    false,
		}, {
			Description: "another database",
			First:       "CREATE TABLE db.flows (`SrcAS` UInt32) ENGINE = Null",
			Second:      "CREATE TABLE other.flows (`SrcAS` UInt32) ENGINE = Null",
			Expected:    false,
		}, {
			Description: "a quoted name is not the same column as a bare one",
			First:       "CREATE TABLE flows (`Src Addr` IPv6) ENGINE = Null",
			Second:      "CREATE TABLE flows (Src Addr) ENGINE = Null",
			Expected:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			first, ok := canonical(tc.First)
			if !ok {
				t.Fatalf("canonical(%q) could not parse", tc.First)
			}
			second, ok := canonical(tc.Second)
			if !ok {
				t.Fatalf("canonical(%q) could not parse", tc.Second)
			}
			if diff := helpers.Diff(first == second, tc.Expected); diff != "" {
				t.Errorf("comparison of the two statements (-got, +want):\n%s\nfirst:  %s\nsecond: %s",
					diff, first, second)
			}
		})
	}
}

// TestOnClusterUnsupported checks a statement taking no ON CLUSTER clause is
// left as is. No builder produces one, so the node is wrapped by hand.
func TestOnClusterUnsupported(t *testing.T) {
	query := Select(Column("SrcAS")).From(Table("flows"))
	got := statement{node: query.query}.OnCluster("akvorado")
	if diff := helpers.Diff(got.String(), query.String()); diff != "" {
		t.Errorf("OnCluster() (-got, +want):\n%s", diff)
	}
}

func TestCanonicalErrors(t *testing.T) {
	cases := []struct {
		Description string
		SQL         string
	}{
		{
			Description: "not SQL at all",
			SQL:         "hello world",
		}, {
			Description: "truncated statement",
			SQL:         "CREATE TABLE flows (`SrcAS` UInt32",
		}, {
			Description: "truncated alias expression",
			SQL:         "CREATE TABLE flows (`SrcAS` UInt32 ALIAS toUInt32(",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if _, ok := canonical(tc.SQL); ok {
				t.Errorf("canonical(%q) parsed", tc.SQL)
			}
		})
	}
}
