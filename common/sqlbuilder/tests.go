// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package sqlbuilder

import (
	"testing"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
	"go.uber.org/mock/gomock"
)

// CheckStatement checks that a complete SQL statement parses. The builder
// produces a syntax tree, so this tells if the formatter turns it back into SQL
// a database would accept.
func CheckStatement(t testing.TB, sql string) {
	t.Helper()
	if _, err := parse(sql); err != nil {
		t.Errorf("cannot parse SQL:\n%s\n\nerror:\n%+v", sql, err)
	}
}

// Normalize parses a SQL statement and renders it again. Two statements that
// only differ by their layout give the same result.
func Normalize(t testing.TB, sql string) string {
	t.Helper()
	if _, err := parse(sql); err != nil {
		t.Fatalf("cannot parse SQL:\n%s\n\nerror:\n%+v", sql, err)
	}
	return normalize(sql)
}

// NormalizeAll is Normalize on several statements.
func NormalizeAll(t testing.TB, sqls []string) []string {
	t.Helper()
	normalized := make([]string, len(sqls))
	for i, sql := range sqls {
		normalized[i] = Normalize(t, sql)
	}
	return normalized
}

// normalize is Normalize with nothing to report an error to. A statement that
// does not parse is returned as is.
func normalize(sql string) string {
	statements, err := parse(sql)
	if err != nil {
		return sql
	}
	formatter := parser.NewFormatter().WithBeautify()
	for _, statement := range statements {
		parser.Walk(statement, dropExtraParens)
		formatter.WriteExpr(statement)
	}
	return formatter.String()
}

// SQLMatcher matches a SQL statement, whatever its layout. It is meant for the
// mocked ClickHouse connection.
func SQLMatcher(t testing.TB, sql string) gomock.Matcher {
	t.Helper()
	return sqlMatcher{expected: Normalize(t, sql)}
}

type sqlMatcher struct {
	expected string
}

func (m sqlMatcher) Matches(x any) bool {
	sql, ok := x.(string)
	return ok && normalize(sql) == m.expected
}

func (m sqlMatcher) String() string {
	return m.expected
}
