// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"strings"

	"akvorado/common/schema"
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

func selectSankeyRowsByLimitType(input graphSankeyHandlerInput, dimensions []string, where string) string {
	return selectRowsByLimitType(input.graphCommonHandlerInput, dimensions, where)
}

func selectLineRowsByLimitType(input graphLineHandlerInput, dimensions []string, where string) string {
	return selectRowsByLimitType(input.graphCommonHandlerInput, dimensions, where)
}

func selectRowsByLimitType(input graphCommonHandlerInput, dimensions []string, where string) string {
	var rowsType string
	var source string
	var orderBy string
	if input.LimitType == "max" {
		source = fmt.Sprintf("( SELECT %s AS sum_at_time FROM source WHERE %s GROUP BY %s )",
			strings.Join(append(dimensions, "{{ .Units }}"), ", "),
			where,
			strings.Join(dimensions, ", "),
		)
		orderBy = "MAX(sum_at_time)"
	} else {
		source = fmt.Sprintf("source WHERE %s", where)
		orderBy = "{{ .Units }}"
	}
	rowsType = fmt.Sprintf(
		"rows AS (SELECT %s FROM %s GROUP BY %s ORDER BY %s DESC LIMIT %d)",
		strings.Join(dimensions, ", "),
		source,
		strings.Join(dimensions, ", "),
		orderBy,
		input.Limit)
	return rowsType
}
