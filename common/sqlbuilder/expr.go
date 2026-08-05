// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package sqlbuilder builds ClickHouse SQL as a syntax tree instead of text. It
// is a thin layer on top of github.com/AfterShip/clickhouse-sql-parser: the
// constructors here return nodes of its AST and the final SQL comes from its
// formatter.
//
// A package needing its own building blocks defines them as transforms and
// passes them to Expr.Apply or Query.Apply.
package sqlbuilder

import (
	"fmt"
	"strings"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// Expr is a SQL expression. Its zero value means there is no expression and the
// helpers below skip it instead of producing invalid SQL.
type Expr struct {
	node parser.Expr
}

// wrap builds an expression from a parser node.
func wrap(node parser.Expr) Expr {
	return Expr{node: node}
}

// nodes unwraps a list of expressions.
func nodes(exprs []Expr) []parser.Expr {
	result := make([]parser.Expr, len(exprs))
	for i, expr := range exprs {
		result[i] = expr.node
	}
	return result
}

// IsZero tells if there is no expression.
func (e Expr) IsZero() bool {
	return e.node == nil
}

// Apply passes the expression through the provided transforms, in order. Other
// packages use it to add their own building blocks on top of this package. The
// same expression can appear in several places of a query, so a transform has
// to build a new expression around the one it gets.
func (e Expr) Apply(transforms ...func(Expr) Expr) Expr {
	for _, transform := range transforms {
		e = transform(e)
	}
	return e
}

// String renders the expression as a single line of SQL.
func (e Expr) String() string {
	if e.IsZero() {
		return ""
	}
	return parser.Format(e.node)
}

// pretty renders the expression as indented, multi-line SQL.
func (e Expr) pretty() string {
	if e.IsZero() {
		return ""
	}
	formatter := parser.NewFormatter().WithBeautify()
	formatter.WriteExpr(e.node)
	return formatter.String()
}

// Column returns a reference to a column.
func Column(name string) Expr {
	return wrap(ident(name))
}

// Columns returns references to several columns.
func Columns(names ...string) []Expr {
	exprs := make([]Expr, len(names))
	for i, name := range names {
		exprs[i] = Column(name)
	}
	return exprs
}

var stringEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// String returns a string literal. The value is escaped, so it is safe to use
// with data coming from a user.
func String(value string) Expr {
	return wrap(&parser.StringLiteral{Literal: stringEscaper.Replace(value)})
}

// Uint returns an unsigned integer literal.
func Uint(value uint64) Expr {
	return wrap(&parser.NumberLiteral{Literal: fmt.Sprintf("%d", value)})
}

// Int returns a signed integer literal.
func Int(value int64) Expr {
	return wrap(&parser.NumberLiteral{Literal: fmt.Sprintf("%d", value)})
}

// Number returns a numeric literal written as is, for example a hexadecimal
// one.
func Number(literal string) Expr {
	return wrap(&parser.NumberLiteral{Literal: literal})
}

// Function returns a call to the named function.
func Function(name string, args ...Expr) Expr {
	return wrap(&parser.FunctionExpr{
		Name:   ident(name),
		Params: &parser.ParamExprList{Items: &parser.ColumnExprList{Items: nodes(args)}},
	})
}

// Lambda returns a lambda function, as in "c -> bitAnd(c, 0xffff)".
func Lambda(parameter string, body Expr) Expr {
	return Op(Column(parameter), "->", body)
}

// Op combines two expressions with a binary operator, for example "=", "<",
// "LIKE" or "IN". A "NOT " prefix on the operator is understood.
func Op(left Expr, operator string, right Expr) Expr {
	hasNot := false
	if rest, found := strings.CutPrefix(operator, "NOT "); found {
		hasNot = true
		operator = rest
	}
	return wrap(&parser.BinaryOperation{
		LeftExpr:  left.node,
		Operation: parser.TokenKind(operator),
		RightExpr: right.node,
		HasNot:    hasNot,
	})
}

// Cast returns the expression cast to the provided ClickHouse type, using the
// postfix "::" form.
func Cast(expr Expr, typeName string) Expr {
	return Op(expr, "::", wrap(rawType(typeName)))
}

// And combines expressions with AND. Empty expressions are ignored. An operand
// which is an OR gets parentheses, as OR binds less tightly.
func And(exprs ...Expr) Expr {
	return combine("AND", exprs)
}

// Or combines expressions with OR. Empty expressions are ignored.
func Or(exprs ...Expr) Expr {
	return combine("OR", exprs)
}

func combine(operator string, exprs []Expr) Expr {
	var result Expr
	for _, expr := range exprs {
		if expr.IsZero() {
			continue
		}
		if operator == "AND" {
			expr = parenthesizeOr(expr)
		}
		if result.IsZero() {
			result = expr
			continue
		}
		result = Op(result, operator, expr)
	}
	return result
}

// parenthesizeOr wraps an OR expression in parentheses. Anything else is left
// alone.
func parenthesizeOr(expr Expr) Expr {
	if binary, ok := expr.node.(*parser.BinaryOperation); ok && binary.Operation == parser.TokenKind("OR") {
		return Parens(expr)
	}
	return expr
}

// Not negates an expression. A compound expression gets parentheses.
func Not(expr Expr) Expr {
	switch expr.node.(type) {
	case *parser.BinaryOperation, *parser.BetweenClause, *parser.UnaryExpr:
		expr = Parens(expr)
	}
	return wrap(&parser.UnaryExpr{Kind: parser.TokenKind("NOT"), Expr: expr.node})
}

// Parens wraps an expression in parentheses.
func Parens(expr Expr) Expr {
	return Tuple(expr)
}

// Tuple returns a parenthesized list of expressions. With a single item, this
// is the same as Parens.
func Tuple(items ...Expr) Expr {
	return wrap(&parser.ParamExprList{
		Items: &parser.ColumnExprList{Items: nodes(items)},
	})
}

// Array returns an array literal.
func Array(items ...Expr) Expr {
	return wrap(&parser.ArrayParamList{
		Items: &parser.ColumnExprList{Items: nodes(items)},
	})
}

// Index reads one element of an array, as in "[a, b][num]".
func Index(array, index Expr) Expr {
	return wrap(&parser.ObjectParams{
		Object: array.node,
		Params: &parser.ArrayParamList{
			Items: &parser.ColumnExprList{Items: nodes([]Expr{index})},
		},
	})
}

// Between returns a BETWEEN condition.
func Between(expr, low, high Expr) Expr {
	return wrap(&parser.BetweenClause{Expr: expr.node, Between: low.node, And: high.node})
}

// NotBetween returns a NOT BETWEEN condition.
func NotBetween(expr, low, high Expr) Expr {
	return wrap(&parser.BetweenClause{
		Expr: expr.node, Not: true, Between: low.node, And: high.node,
	})
}

// Interval returns an INTERVAL expression, for example "INTERVAL 60 second".
func Interval(value Expr, unit string) Expr {
	// IntervalPos must not be zero, otherwise the INTERVAL keyword is dropped.
	return wrap(&parser.IntervalExpr{
		IntervalPos: 1, Expr: value.node, Unit: ident(unit),
	})
}

// Alias names an expression with AS.
func Alias(expr Expr, name string) Expr {
	return wrap(&parser.AliasExpr{Expr: expr.node, Alias: ident(name)})
}

// Star returns the "*" of "SELECT *".
func Star() Expr {
	return wrap(&parser.Ident{Name: "*"})
}

// Modifier changes what a select item covers, like the "EXCEPT (a, b)" of
// "SELECT * EXCEPT (a, b)".
type Modifier struct {
	node *parser.FunctionExpr
}

func modifier(name string, items []Expr) Modifier {
	return Modifier{node: &parser.FunctionExpr{
		Name:   ident(name),
		Params: &parser.ParamExprList{Items: &parser.ColumnExprList{Items: nodes(items)}},
	}}
}

// Replace is the REPLACE modifier of "SELECT * REPLACE (…)". Each item should
// be an aliased expression.
func Replace(items ...Expr) Modifier {
	return modifier("REPLACE", items)
}

// Except is the EXCEPT modifier of "SELECT * EXCEPT (…)".
func Except(columns ...string) Modifier {
	return modifier("EXCEPT", Columns(columns...))
}
