// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
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

func metricForTopSort(inputUnit string) string {
	switch inputUnit {
	case "pps":
		return "Packets"
	default:
		return "Bytes"
	}
}
