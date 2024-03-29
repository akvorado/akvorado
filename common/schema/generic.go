// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import "strings"

// LookupColumnByName can lookup a column by its name.
func (schema *Schema) LookupColumnByName(name string) (*Column, bool) {
	key, ok := columnNameMap.LoadKey(name)
	if !ok {
		return &Column{}, false
	}
	return schema.LookupColumnByKey(key)
}

// LookupColumnByKey can lookup a column by its key.
func (schema *Schema) LookupColumnByKey(key ColumnKey) (*Column, bool) {
	column := schema.columnIndex[key]
	if column == nil {
		return &Column{}, false
	}
	return column, true
}

// ReverseColumnDirection reverts the direction of a provided column name.
func (schema *Schema) ReverseColumnDirection(key ColumnKey) ColumnKey {
	var candidateName string
	name := key.String()
	if strings.HasPrefix(name, "Src") {
		candidateName = "Dst" + name[3:]
	}
	if strings.HasPrefix(name, "Dst") {
		candidateName = "Src" + name[3:]
	}
	if strings.HasPrefix(name, "In") {
		candidateName = "Out" + name[2:]
	}
	if strings.HasPrefix(name, "Out") {
		candidateName = "In" + name[3:]
	}
	if candidateKey, ok := columnNameMap.LoadKey(candidateName); ok {
		if column, ok := schema.LookupColumnByKey(candidateKey); ok && !column.Disabled {
			return candidateKey
		}
	}
	return key
}

// Columns returns the columns.
func (schema *Schema) Columns() []Column {
	columns := make([]Column, 0, len(schema.columns))
	for _, column := range schema.columns {
		if !column.Disabled {
			columns = append(columns, column)
		}
	}
	return columns
}

// IsDisabled tells if a column group is disabled.
func (schema *Schema) IsDisabled(group ColumnGroup) bool {
	return schema.disabledGroups.Test(uint(group))
}
