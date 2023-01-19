// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

// ClickHouseDefinition turns a column into a declaration for ClickHouse
func (column Column) ClickHouseDefinition() string {
	result := []string{fmt.Sprintf("`%s`", column.Name), column.ClickHouseType}
	if column.ClickHouseCodec != "" {
		result = append(result, fmt.Sprintf("CODEC(%s)", column.ClickHouseCodec))
	}
	if column.ClickHouseAlias != "" {
		result = append(result, fmt.Sprintf("ALIAS %s", column.ClickHouseAlias))
	}
	return strings.Join(result, " ")
}

// ClickHouseTableOption is an option to alter the values returned by ClickHouseCreateTable() and ClickHouseSelectColumns().
type ClickHouseTableOption int

const (
	// ClickHouseSkipMainOnlyColumns skips the columns for the main flows table only.
	ClickHouseSkipMainOnlyColumns ClickHouseTableOption = iota
	// ClickHouseSkipGeneratedColumns skips the columns with a GenerateFrom value
	ClickHouseSkipGeneratedColumns
	// ClickHouseSkipTransformColumns skips the columns with a TransformFrom value
	ClickHouseSkipTransformColumns
	// ClickHouseSkipAliasedColumns skips the columns with a Alias value
	ClickHouseSkipAliasedColumns
	// ClickHouseSkipTimeReceived skips the time received column
	ClickHouseSkipTimeReceived
	// ClickHouseUseTransformFromType uses the type from TransformFrom if any
	ClickHouseUseTransformFromType
	// ClickHouseSubstituteGenerates changes the column name to use the default generated value
	ClickHouseSubstituteGenerates
	// ClickHouseSubstituteTransforms changes the column name to use the transformed value
	ClickHouseSubstituteTransforms
)

// ClickHouseCreateTable returns the columns for the CREATE TABLE clause in ClickHouse.
func (schema Schema) ClickHouseCreateTable(options ...ClickHouseTableOption) string {
	lines := []string{}
	schema.clickhouseIterate(func(column Column) {
		lines = append(lines, column.ClickHouseDefinition())
	}, options...)
	return strings.Join(lines, ",\n")
}

// ClickHouseSelectColumns returns the columns matching the options for use in SELECT
func (schema Schema) ClickHouseSelectColumns(options ...ClickHouseTableOption) []string {
	cols := []string{}
	schema.clickhouseIterate(func(column Column) {
		cols = append(cols, column.Name)
	}, options...)
	return cols
}

func (schema Schema) clickhouseIterate(fn func(Column), options ...ClickHouseTableOption) {
	for _, column := range schema.Columns() {
		if slices.Contains(options, ClickHouseSkipTimeReceived) && column.Key == ColumnTimeReceived {
			continue
		}
		if slices.Contains(options, ClickHouseSkipMainOnlyColumns) && column.ClickHouseMainOnly {
			continue
		}
		if slices.Contains(options, ClickHouseSkipGeneratedColumns) && column.ClickHouseGenerateFrom != "" {
			continue
		}
		if slices.Contains(options, ClickHouseSkipTransformColumns) && column.ClickHouseTransformFrom != nil {
			continue
		}
		if slices.Contains(options, ClickHouseSkipAliasedColumns) && column.ClickHouseAlias != "" {
			continue
		}
		if slices.Contains(options, ClickHouseUseTransformFromType) && column.ClickHouseTransformFrom != nil {
			for _, ocol := range column.ClickHouseTransformFrom {
				// We assume we only need to use name/type
				column.Name = ocol.Name
				column.ClickHouseType = ocol.ClickHouseType
				fn(column)
			}
			continue
		}
		if slices.Contains(options, ClickHouseSubstituteGenerates) && column.ClickHouseGenerateFrom != "" {
			column.Name = fmt.Sprintf("%s AS %s", column.ClickHouseGenerateFrom, column.Name)
		}
		if slices.Contains(options, ClickHouseSubstituteTransforms) && column.ClickHouseTransformFrom != nil {
			column.Name = fmt.Sprintf("%s AS %s", column.ClickHouseTransformTo, column.Name)
		}
		fn(column)
	}
}

// ClickHouseSortingKeys returns the list of sorting keys, prefixed by the primary keys.
func (schema Schema) ClickHouseSortingKeys() []string {
	cols := schema.ClickHousePrimaryKeys()
	for _, column := range schema.Columns() {
		if column.ClickHouseNotSortingKey || column.ClickHouseMainOnly {
			continue
		}
		if !slices.Contains(cols, column.Name) {
			cols = append(cols, column.Name)
		}
	}
	return cols
}

// ClickHousePrimaryKeys returns the list of primary keys.
func (schema Schema) ClickHousePrimaryKeys() []string {
	cols := []string{}
	for _, key := range schema.clickHousePrimaryKeys {
		cols = append(cols, key.String())
	}
	return cols
}
