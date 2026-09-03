// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// parse turns SQL into statements. The underlying parser panics on a few
// inputs instead of reporting an error. As some of the SQL parsed here comes
// from ClickHouse itself, a panic is turned into an error.
func parse(sql string) (statements []parser.Expr, err error) {
	defer func() {
		if problem := recover(); problem != nil {
			statements, err = nil, fmt.Errorf("cannot parse SQL: %v", problem)
		}
	}()
	return parser.NewParser(sql).ParseStmts()
}

// canonical rewrites SQL into a single form, so two statements written
// differently can be compared. It also tells if the SQL could be parsed.
func canonical(sql string) (string, bool) {
	statements, err := parse(sql)
	if err != nil {
		return "", false
	}
	formatter := parser.NewFormatter()
	for _, statement := range statements {
		// Parsing keeps the quotes an identifier was written with, so the same
		// statement with and without them does not compare equal. ClickHouse
		// does not quote in the same places as we do, so every identifier is
		// rewritten the way ClickHouse writes it.
		parser.Walk(statement, func(node parser.Expr) bool {
			if identifier, ok := node.(*parser.Ident); ok {
				identifier.QuoteType = quoteType(identifier.Name)
			}
			dropExtraParens(node)
			return true
		})
		formatter.WriteExpr(statement)
	}
	return formatter.String(), true
}

// dropExtraParens removes the parentheses ClickHouse keeps or not depending on
// its version: CAST((96 + Mask), 'UInt8') is CAST(96 + Mask, 'UInt8'). Only a
// function argument and the value of a CAST are touched, they are already
// delimited. Elsewhere, parentheses can set the order of the operators.
func dropExtraParens(node parser.Expr) bool {
	switch node := node.(type) {
	case *parser.ColumnExpr:
		node.Expr = withoutParens(node.Expr)
	case *parser.CastExpr:
		node.Expr = withoutParens(node.Expr)
	}
	return true
}

// withoutParens returns what a pair of parentheses wraps. A tuple is kept.
func withoutParens(node parser.Expr) parser.Expr {
	for {
		group, ok := node.(*parser.ParamExprList)
		if !ok || len(group.Items.Items) != 1 {
			return node
		}
		item, ok := group.Items.Items[0].(*parser.ColumnExpr)
		if !ok || item.Alias != nil {
			return node
		}
		node = item.Expr
	}
}

// StripTableSettings removes the SETTINGS clause from the CREATE TABLE
// statements it is given.
func StripTableSettings(sql string) string {
	statements, err := parse(sql)
	if err != nil {
		return sql
	}
	formatter := parser.NewFormatter()
	for _, statement := range statements {
		if create, ok := statement.(*parser.CreateTable); ok && create.Engine != nil {
			create.Engine.Settings = nil
		}
		formatter.WriteExpr(statement)
	}
	return formatter.String()
}

// ParseExpr parses a SQL expression. The parser only accepts complete
// statements, so the expression is parsed as the select list of a SELECT. This
// is used in widgets, so we need to keep this function accessible outside of
// tests.
func ParseExpr(sql string) (Expr, error) {
	statements, err := parse(fmt.Sprintf("SELECT %s", sql))
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

// ParseExprs parses several SQL expressions.
func ParseExprs(sqls []string) ([]Expr, error) {
	exprs := make([]Expr, len(sqls))
	for i, sql := range sqls {
		expr, err := ParseExpr(sql)
		if err != nil {
			return nil, err
		}
		exprs[i] = expr
	}
	return exprs, nil
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
