// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

// String turns a column into a declaration for ClickHouse
func (column Column) String() string {
	result := []string{fmt.Sprintf("`%s`", column.Name), column.Type}
	if column.Codec != "" {
		result = append(result, fmt.Sprintf("CODEC(%s)", column.Codec))
	}
	if column.Alias != "" {
		result = append(result, fmt.Sprintf("ALIAS %s", column.Alias))
	}
	return strings.Join(result, " ")
}

// TableOption is an option to alter the values returned by Table() and Columns().
type TableOption int

const (
	// SkipMainOnlyColumns skips the columns for the main flows table only.
	SkipMainOnlyColumns TableOption = iota
	// SkipGeneratedColumns skips the columns with a GenerateFrom value
	SkipGeneratedColumns
	// SkipTransformColumns skips the columns with a TransformFrom value
	SkipTransformColumns
	// SkipAliasedColumns skips the columns with a Alias value
	SkipAliasedColumns
	// SkipTimeReceived skips the time received column
	SkipTimeReceived
	// UseTransformFromType uses the type from TransformFrom if any
	UseTransformFromType
	// SubstituteGenerates changes the column name to use the default generated value
	SubstituteGenerates
	// SubstituteTransforms changes the column name to use the transformed value
	SubstituteTransforms
)

// CreateTable returns the columns for the CREATE TABLE clause in ClickHouse.
func (schema Schema) CreateTable(options ...TableOption) string {
	lines := []string{}
	schema.iterate(func(column Column) {
		lines = append(lines, column.String())
	}, options...)
	return strings.Join(lines, ",\n")
}

// SelectColumns returns the column for the SELECT clause in ClickHouse.
func (schema Schema) SelectColumns(options ...TableOption) []string {
	cols := []string{}
	schema.iterate(func(column Column) {
		cols = append(cols, column.Name)
	}, options...)
	return cols
}

func (schema Schema) iterate(fn func(column Column), options ...TableOption) {
	for _, column := range schema.Columns {
		if slices.Contains(options, SkipTimeReceived) && column.Name == "TimeReceived" {
			continue
		}
		if slices.Contains(options, SkipMainOnlyColumns) && column.MainOnly {
			continue
		}
		if slices.Contains(options, SkipGeneratedColumns) && column.GenerateFrom != "" {
			continue
		}
		if slices.Contains(options, SkipTransformColumns) && column.TransformFrom != nil {
			continue
		}
		if slices.Contains(options, SkipAliasedColumns) && column.Alias != "" {
			continue
		}
		if slices.Contains(options, UseTransformFromType) && column.TransformFrom != nil {
			for _, ocol := range column.TransformFrom {
				// We assume we only need to use name/type
				column.Name = ocol.Name
				column.Type = ocol.Type
				fn(column)
			}
			continue
		}
		if slices.Contains(options, SubstituteGenerates) && column.GenerateFrom != "" {
			column.Name = fmt.Sprintf("%s AS %s", column.GenerateFrom, column.Name)
		}
		if slices.Contains(options, SubstituteTransforms) && column.TransformFrom != nil {
			column.Name = fmt.Sprintf("%s AS %s", column.TransformTo, column.Name)
		}
		fn(column)
	}
}

// SortingKeys returns the list of sorting keys, prefixed by the primary keys.
func (schema Schema) SortingKeys() []string {
	cols := append([]string{}, schema.PrimaryKeys...)
	for _, column := range schema.Columns {
		if column.NotSortingKey || column.MainOnly {
			continue
		}
		if !slices.Contains(cols, column.Name) {
			cols = append(cols, column.Name)
		}
	}
	return cols
}
