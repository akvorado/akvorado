// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"slices"
	"strings"

	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

func requireMainTable(sch *schema.Component, qcs []query.Column, qf query.Filter) bool {
	if qf.MainTableRequired() {
		return true
	}
	for _, qc := range qcs {
		if column, ok := sch.LookupColumnByKey(qc.Key()); ok && column.ClickHouseMainOnly {
			return true
		}
	}
	return false
}

// fixQueryColumnName fix capitalization of the provided column name
func (c *Component) fixQueryColumnName(name string) string {
	name = strings.ToLower(name)
	for _, column := range c.d.Schema.Columns() {
		if strings.ToLower(column.Name) == name {
			return column.Name
		}
	}
	return ""
}

func selectSankeyRowsByLimitType(input graphSankeyHandlerInput, r resolved, dimensions []sb.Expr, where, units sb.Expr) *sb.Query {
	return selectRowsByLimitType(input.graphCommonHandlerInput, r, dimensions, where, units)
}

func selectLineRowsByLimitType(input graphLineHandlerInput, r resolved, dimensions []sb.Expr, where, units sb.Expr) *sb.Query {
	return selectRowsByLimitType(input.graphCommonHandlerInput, r, dimensions, where, units)
}

// selectRowsByLimitType builds the query picking the dimension values to
// display. The other values are grouped as "Other".
func selectRowsByLimitType(input graphCommonHandlerInput, r resolved, dimensions []sb.Expr, where, units sb.Expr) *sb.Query {
	rows := sb.Select(dimensions...).GroupBy(dimensions...).Limit(input.Limit)
	if input.LimitType == "max" {
		// Rank on the highest value reached during one interval instead of the
		// total, so a short burst is not hidden by a steady flow. The inner
		// query groups on time as well: without it, each dimension gets a
		// single row holding its total and the maximum is that total.
		rows.FromSelect(
			sb.Select(slices.Concat(
				[]sb.Expr{sb.Alias(r.toStartOfInterval(), "time")},
				dimensions,
				[]sb.Expr{sb.Alias(units, "sum_at_time")})...).
				From(sb.Table("source")).
				Where(where).
				GroupBy(slices.Concat(
					[]sb.Expr{sb.Column("time")}, dimensions)...)).
			OrderBy(sb.Order(
				sb.Function("MAX", sb.Column("sum_at_time"))).Desc())
		return rows
	}
	if input.LimitType == "last" {
		// Rank on the traffic of the last interval only, so the rows are the
		// ones busy at the end of the range. Every dimension is measured on the
		// same window, so they stay comparable.
		return rows.From(sb.Table("source")).
			Where(sb.And(where,
				sb.Op(sb.Column("TimeReceived"), ">=",
					sb.Op(r.timefilterEnd(), "-", seconds(r.Interval))))).
			OrderBy(sb.Order(units).Desc())
	}
	// On the main table, an exact ranking has to keep one entry for each
	// dimension value and there can be millions of them, like when asking for
	// the top source addresses. topKWeighted keeps a bounded number of
	// candidates instead. Its result is approximate, so it stays reserved for
	// the table where the exact ranking is expensive. A limit of 0 is refused by
	// topKWeighted, while the exact query returns nothing and puts everything
	// in "Other".
	if weight := unitsWeightExpr(input.Units); r.Table == "flows" &&
		input.Limit > 0 && !weight.IsZero() {
		return selectRowsByTopK(input, dimensions, where, weight)
	}
	return rows.From(sb.Table("source")).Where(where).OrderBy(sb.Order(units).Desc())
}

// topKWeightedLoadFactor is how many candidates topKWeighted keeps for each
// requested row. The default of 3 loses a few of the real top values once the
// limit reaches a few tens.
const topKWeightedLoadFactor = 20

// selectRowsByTopK ranks the dimension values with an approximate top-N. It
// produces the same shape as the exact query, one column per dimension, so the
// callers query "rows" the same way.
func selectRowsByTopK(input graphCommonHandlerInput, dimensions []sb.Expr, where, weight sb.Expr) *sb.Query {
	// topKWeighted returns an array of tuples. Unrolling it gives back one row
	// per dimension value.
	top := sb.Select(
		sb.Alias(
			sb.Function("arrayJoin",
				sb.ParametricFunction("topKWeighted",
					[]sb.Expr{sb.Int(int64(input.Limit)), sb.Uint(topKWeightedLoadFactor)},
					sb.Function("tuple", dimensions...),
					sb.Function("toUInt64", weight))),
			"tk")).
		From(sb.Table("source")).
		Where(where)
	fields := make([]sb.Expr, 0, len(input.Dimensions))
	for idx, column := range input.Dimensions {
		fields = append(fields, sb.Alias(
			sb.Function("tupleElement", sb.Column("tk"), sb.Uint(uint64(idx+1))),
			column.String()))
	}
	return sb.Select(fields...).FromSelect(top)
}
