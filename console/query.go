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

func selectSankeyRowsByLimitType(input graphSankeyHandlerInput, dimensions []sb.Expr, where, units sb.Expr) *sb.Query {
	return selectRowsByLimitType(input.graphCommonHandlerInput, dimensions, where, units)
}

func selectLineRowsByLimitType(input graphLineHandlerInput, dimensions []sb.Expr, where, units sb.Expr) *sb.Query {
	return selectRowsByLimitType(input.graphCommonHandlerInput, dimensions, where, units)
}

// selectRowsByLimitType builds the query picking the dimension values to
// display. The other values are grouped as "Other".
func selectRowsByLimitType(input graphCommonHandlerInput, dimensions []sb.Expr, where, units sb.Expr) *sb.Query {
	rows := sb.Select(dimensions...).GroupBy(dimensions...).Limit(input.Limit)
	if input.LimitType == "max" {
		// Rank on the highest value reached instead of the total, so a short
		// burst is not hidden by a steady flow.
		rows.FromSelect(
			sb.Select(slices.Concat(dimensions,
				[]sb.Expr{sb.Alias(units, "sum_at_time")})...).
				From("source").
				Where(where).
				GroupBy(dimensions...)).
			OrderBy(sb.Order(
				sb.Function("MAX", sb.Column("sum_at_time"))).Desc())
		return rows
	}
	return rows.From("source").Where(where).OrderBy(sb.Order(units).Desc())
}
