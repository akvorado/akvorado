// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"fmt"
	"testing"

	chparser "github.com/AfterShip/clickhouse-sql-parser/parser"
)

// checkWhereParses checks an expression is a valid boolean expression.
func checkWhereParses(t *testing.T, where string) {
	t.Helper()
	if where == "" {
		return
	}
	sql := fmt.Sprintf("SELECT 1 FROM flows WHERE %s", where)
	if _, err := chparser.NewParser(sql).ParseStmts(); err != nil {
		t.Errorf("ParseStmts(%q) error:\n%+v", where, err)
	}
}
