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
	return rows.From(sb.Table("source")).Where(where).OrderBy(sb.Order(units).Desc())
}
