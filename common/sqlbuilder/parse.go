// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// ParseExpr parses a SQL expression. The parser only accepts complete
// statements, so the expression is parsed as the select list of a SELECT. This
// is used in widgets, so we need to keep this function accessible outside of
// tests.
func ParseExpr(sql string) (Expr, error) {
	statements, err := parser.NewParser(fmt.Sprintf("SELECT %s", sql)).ParseStmts()
	if err != nil {
		return Expr{}, err
	}
	if len(statements) != 1 {
		return Expr{}, fmt.Errorf("expected one statement, got %d", len(statements))
	}
	query, ok := statements[0].(*parser.SelectQuery)
	if !ok {
		return Expr{}, errors.New("not an expression")
	}
	if len(query.SelectItems) != 1 {
		return Expr{}, fmt.Errorf("expected one expression, got %d", len(query.SelectItems))
	}
	if !onlySelectItems(query) {
		return Expr{}, errors.New("trailing text after the expression")
	}
	item := query.SelectItems[0]
	if item.Alias != nil {
		return wrap(&parser.AliasExpr{Expr: item.Expr, Alias: item.Alias}), nil
	}
	return wrap(item.Expr), nil
}

// onlySelectItems tells if a SELECT carries nothing but its select list.
// Anything written after the expression lands in one of the other clauses,
// where it would be dropped without notice. Fields are walked instead of being
// listed one by one, so a new clause in the parser cannot slip through.
func onlySelectItems(query *parser.SelectQuery) bool {
	value := reflect.ValueOf(*query)
	for i := range value.NumField() {
		switch value.Type().Field(i).Name {
		case "SelectPos", "StatementEnd", "SelectItems":
			continue
		}
		if !value.Field(i).IsZero() {
			return false
		}
	}
	return true
}

// MustParseExpr parses a SQL expression written in the code. It panics when the
// expression is invalid. Never use it on user input, use ParseExpr instead.
func MustParseExpr(sql string) Expr {
	expr, err := ParseExpr(sql)
	if err != nil {
		panic(fmt.Sprintf("invalid SQL expression %q: %s", sql, err))
	}
	return expr
}
