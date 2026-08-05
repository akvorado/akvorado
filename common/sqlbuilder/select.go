// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// Query is a SELECT statement under construction. Its methods return the query
// itself, so they can be chained.
type Query struct {
	query *parser.SelectQuery
}

// Select starts a SELECT statement with the provided items.
func Select(items ...Expr) *Query {
	q := &Query{query: &parser.SelectQuery{}}
	for _, item := range items {
		q.Item(item)
	}
	return q
}

// Apply passes the query through the provided transforms, in order. Other
// packages use it to add their own building blocks on top of this package.
func (q *Query) Apply(transforms ...func(*Query) *Query) *Query {
	for _, transform := range transforms {
		q = transform(q)
	}
	return q
}

// Item appends one item to the select list. Modifiers apply to that item, for
// example Replace() on a Star().
func (q *Query) Item(expr Expr, modifiers ...Modifier) *Query {
	item := &parser.SelectItem{Expr: expr.node}
	for _, modifier := range modifiers {
		item.Modifiers = append(item.Modifiers, modifier.node)
	}
	q.query.SelectItems = append(q.query.SelectItems, item)
	return q
}

// From reads from the named table.
func (q *Query) From(table TableName) *Query {
	q.query.From = &parser.FromClause{
		Expr: &parser.TableExpr{Expr: table.node()},
	}
	return q
}

// FromSelect reads from a subquery.
func (q *Query) FromSelect(sub *Query) *Query {
	q.query.From = &parser.FromClause{
		Expr: &parser.TableExpr{
			Expr: &parser.SubQuery{HasParen: true, Select: sub.query},
		},
	}
	return q
}

// ArrayJoin unrolls an array into rows. From must be called first.
func (q *Query) ArrayJoin(expr Expr) *Query {
	q.query.From.Expr = &parser.JoinExpr{
		Left:  q.query.From.Expr,
		Right: &parser.JoinExpr{Modifiers: []string{"ARRAY", "JOIN"}, Left: expr.node},
	}
	return q
}

// Where sets the WHERE clause. An empty expression removes it.
func (q *Query) Where(expr Expr) *Query {
	if expr.IsZero() {
		q.query.Where = nil
		return q
	}
	q.query.Where = &parser.WhereClause{Expr: expr.node}
	return q
}

// GroupBy sets the GROUP BY clause.
func (q *Query) GroupBy(exprs ...Expr) *Query {
	if len(exprs) == 0 {
		q.query.GroupBy = nil
		return q
	}
	q.query.GroupBy = &parser.GroupByClause{
		Expr: &parser.ColumnExprList{Items: nodes(exprs)},
	}
	return q
}

// OrderBy sets the ORDER BY clause.
func (q *Query) OrderBy(items ...OrderItem) *Query {
	if len(items) == 0 {
		q.query.OrderBy = nil
		return q
	}
	exprs := make([]parser.Expr, len(items))
	for i, item := range items {
		exprs[i] = item.node()
	}
	q.query.OrderBy = &parser.OrderByClause{Items: exprs}
	return q
}

// Interpolate adds an item to the INTERPOLATE clause of the ORDER BY. It only
// makes sense together with a WITH FILL, so OrderBy must be called first.
func (q *Query) Interpolate(column string, expr Expr) *Query {
	if q.query.OrderBy.Interpolate == nil {
		q.query.OrderBy.Interpolate = &parser.InterpolateClause{}
	}
	q.query.OrderBy.Interpolate.Items = append(q.query.OrderBy.Interpolate.Items,
		&parser.InterpolateItem{Column: &parser.Ident{Name: column}, Expr: expr.node})
	return q
}

// Limit sets the LIMIT clause.
func (q *Query) Limit(limit int) *Query {
	q.query.Limit = &parser.LimitClause{Limit: Int(int64(limit)).node}
	return q
}

// Setting adds one entry to the SETTINGS clause.
func (q *Query) Setting(name string, value Expr) *Query {
	if q.query.Settings == nil {
		q.query.Settings = &parser.SettingsClause{}
	}
	q.query.Settings.Items = append(q.query.Settings.Items, &parser.SettingExpr{
		Name: &parser.Ident{Name: name},
		Expr: value.node,
	})
	return q
}

// With adds a named CTE, as in "WITH name AS (SELECT …)".
func (q *Query) With(name string, sub *Query) *Query {
	return q.addCTE(&parser.CTEStmt{
		Expr:  &parser.Ident{Name: name},
		Alias: sub.query,
	})
}

// WithScalar adds a scalar CTE, as in "WITH (SELECT …) AS name".
func (q *Query) WithScalar(sub *Query, name string) *Query {
	return q.addCTE(&parser.CTEStmt{
		Expr:  &parser.SubQuery{HasParen: true, Select: sub.query},
		Alias: &parser.Ident{Name: name},
	})
}

// WithAlias names an expression in the WITH clause, as in
// "WITH arrayCompact(DstASPath) AS c_DstASPath".
func (q *Query) WithAlias(expr Expr, name string) *Query {
	return q.addCTE(&parser.CTEStmt{
		Expr:  expr.node,
		Alias: &parser.Ident{Name: name},
	})
}

func (q *Query) addCTE(cte *parser.CTEStmt) *Query {
	if q.query.With == nil {
		q.query.With = &parser.WithClause{}
	}
	q.query.With.CTEs = append(q.query.With.CTEs, cte)
	return q
}

// UnionAll appends another query with UNION ALL. Only the first query of the
// union carries the WITH clause, the others read its CTEs.
func (q *Query) UnionAll(other *Query) *Query {
	last := q.query
	for last.UnionAll != nil {
		last = last.UnionAll
	}
	last.UnionAll = other.query
	return q
}

// Subquery returns the query wrapped in parentheses, for use as an expression.
func (q *Query) Subquery() Expr {
	return wrap(&parser.SubQuery{HasParen: true, Select: q.query})
}

// String renders the query as indented, multi-line SQL.
func (q *Query) String() string {
	return wrap(q.query).pretty()
}

// Matches tells if the provided SQL means the same as the query. Only the
// meaning is compared: layout and identifier quoting do not matter. SQL that
// cannot be parsed never matches.
func (q *Query) Matches(sql string) bool {
	return statement{node: q.query}.Matches(sql)
}

// OrderItem is one item of an ORDER BY clause.
type OrderItem struct {
	expr       Expr
	descending bool
	fill       bool
	from, to   Expr
	step       Expr
}

// Order sorts on an expression, in ascending order.
func Order(expr Expr) OrderItem {
	return OrderItem{expr: expr}
}

// Desc switches the item to descending order.
func (o OrderItem) Desc() OrderItem {
	o.descending = true
	return o
}

// Fill adds a WITH FILL clause to the item.
func (o OrderItem) Fill(from, to, step Expr) OrderItem {
	o.fill = true
	o.from = from
	o.to = to
	o.step = step
	return o
}

func (o OrderItem) node() parser.Expr {
	order := &parser.OrderExpr{
		Expr:      o.expr.node,
		Direction: parser.OrderDirectionNone,
	}
	if o.descending {
		order.Direction = parser.OrderDirectionDesc
	}
	if o.fill {
		order.Fill = &parser.Fill{From: o.from.node, To: o.to.node, Step: o.step.node}
	}
	return order
}
