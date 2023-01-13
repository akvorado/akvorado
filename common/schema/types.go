// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package schema is an abstraction of the data schema used by Akvorado. It is a
// leaky abstraction as there are multiple parts dependant of the subsystem that
// will use it.
package schema

import orderedmap "github.com/elliotchance/orderedmap/v2"

// Schema is the data schema.
type Schema struct {
	// We use an ordered map for direct access to columns.
	Columns *orderedmap.OrderedMap[ColumnKey, Column]

	// For ClickHouse. This is the set of primary keys (order is important and
	// may not follow column order).
	ClickHousePrimaryKeys []ColumnKey
}

// Column represents a column of data.
type Column struct {
	Key      ColumnKey
	Name     string
	MainOnly bool

	// For ClickHouse. `NotSortingKey' is for columns generated from other
	// columns. It is only useful if not MainOnly and not Alias. `GenerateFrom'
	// is for a column that's generated from an SQL expression instead of being
	// retrieved from the protobuf. `TransformFrom' and `TransformTo' work in
	// pairs. The first one is the set of column in the raw table while the
	// second one is how to transform it for the main table.
	ClickHouseType          string
	ClickHouseCodec         string
	ClickHouseAlias         string
	ClickHouseNotSortingKey bool
	ClickHouseGenerateFrom  string
	ClickHouseTransformFrom []Column
	ClickHouseTransformTo   string

	// For the console.
	ConsoleNotDimension bool
}

// ColumnKey is the name of a column
type ColumnKey int
