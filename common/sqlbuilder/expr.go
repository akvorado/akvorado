// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package sqlbuilder builds ClickHouse SQL as a syntax tree instead of text. It
// is a thin layer on top of github.com/AfterShip/clickhouse-sql-parser: the
// constructors here return nodes of its AST and the final SQL comes from its
// formatter.
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

// ParametricFunction returns a call to a function taking parameters on top of
// its arguments, as in "topKWeighted(10, 20)(SrcAddr, Bytes)". Aggregate
// functions are the ones using this form.
func ParametricFunction(name string, parameters []Expr, args ...Expr) Expr {
	return wrap(&parser.FunctionExpr{
		Name: ident(name),
		Params: &parser.ParamExprList{
			Items:         &parser.ColumnExprList{Items: nodes(parameters)},
			ColumnArgList: &parser.ColumnArgList{Items: nodes(args)},
		},
	})
}

// Lambda returns a lambda function, as in "c -> bitAnd(c, 0xffff)".
func Lambda(parameter string, body Expr) Expr {
	return Op(Column(parameter), "->", body)
}

// How tightly the operators bind, from the loosest to the tightest. See
// https://clickhouse.com/docs/sql-reference/operators.
const (
	// precUnknown is for an operator this package does not know about. It binds
	// the loosest, so it gets parentheses rather than silently changing the
	// meaning of a query.
	precUnknown = iota
	precLambda
	precOr
	precAnd
	precNot
	precComparison
	precConcat
	precAddition
	precMultiplication
	precCast
	// precAtom is for what is not an operator at all: a literal, a column, a
	// function call, a parenthesized expression… It never needs parentheses.
	precAtom
)

var precedences = map[string]int{
	"->":     precLambda,
	"OR":     precOr,
	"AND":    precAnd,
	"=":      precComparison,
	"==":     precComparison,
	"!=":     precComparison,
	"<>":     precComparison,
	"<":      precComparison,
	"<=":     precComparison,
	">":      precComparison,
	">=":     precComparison,
	"IN":     precComparison,
	"LIKE":   precComparison,
	"ILIKE":  precComparison,
	"REGEXP": precComparison,
	"IS":     precComparison,
	"||":     precConcat,
	"+":      precAddition,
	"-":      precAddition,
	"*":      precMultiplication,
	"/":      precMultiplication,
	"%":      precMultiplication,
	"::":     precCast,
}

// associative lists the operators where "a OP (b OP c)" and "(a OP b) OP c"
// mean the same, so a right operand using the same operator needs no
// parentheses. The arithmetic ones are left out on purpose: regrouping them
// changes the result with floating point numbers.
var associative = map[string]bool{
	"AND": true,
	"OR":  true,
}

// operatorName normalizes an operator for the two tables above. The parser
// keeps the case an operator was written with, and it keeps the "NOT " prefix
// inside the operator while Op() takes it apart. The prefix does not change how
// tightly an operator binds.
func operatorName(operation parser.TokenKind) string {
	return strings.TrimPrefix(strings.ToUpper(string(operation)), "NOT ")
}

// precedence tells how tightly a node binds, so an operand can be parenthesized
// when the operator above it binds tighter.
func precedence(node parser.Expr) int {
	switch node := node.(type) {
	case *parser.BinaryOperation:
		return precedences[operatorName(node.Operation)]
	case *parser.UnaryExpr:
		return precNot
	case *parser.BetweenClause:
		return precComparison
	}
	return precAtom
}

// Op combines two expressions with a binary operator, for example "=", "<",
// "LIKE" or "IN". A "NOT " prefix on the operator is understood. An operand
// binding less tightly than the operator gets parentheses, so the SQL says what
// the tree says.
func Op(left Expr, operator string, right Expr) Expr {
	hasNot := false
	if rest, found := strings.CutPrefix(operator, "NOT "); found {
		hasNot = true
		operator = rest
	}
	name := operatorName(parser.TokenKind(operator))
	prec := precedences[name]
	// The left operand already reads the way the operators associate: "a - b -
	// c" is "(a - b) - c", so it needs nothing when both bind the same.
	if precedence(left.node) < prec {
		left = Parens(left)
	}
	if operand := precedence(right.node); operand < prec ||
		(operand == prec && !associative[name]) {
		right = Parens(right)
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

// And combines expressions with AND. Empty expressions are ignored.
func And(exprs ...Expr) Expr {
	return combine("AND", exprs)
}

// Or combines expressions with OR. Empty expressions are ignored.
func Or(exprs ...Expr) Expr {
	return combine("OR", exprs)
}

// combine chains expressions with the same operator. Op() takes care of the
// parentheses an operand may need.
func combine(operator string, exprs []Expr) Expr {
	var result Expr
	for _, expr := range exprs {
		if expr.IsZero() {
			continue
		}
		if result.IsZero() {
			result = expr
			continue
		}
		result = Op(result, operator, expr)
	}
	return result
}

// Not negates an expression.
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
