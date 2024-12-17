// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"github.com/ClickHouse/ch-go/proto"
	"github.com/bits-and-blooms/bitset"
)

// Schema is the data schema.
type Schema struct {
	columns        []Column      // Ordered list of columns
	columnIndex    []*Column     // Columns indexed by ColumnKey
	disabledGroups bitset.BitSet // Disabled column groups

	// dynamicColumns is the number of columns that are generated at runtime and appended after columnLast
	dynamicColumns ColumnKey
	// For ClickHouse. This is the set of primary keys (order is important and
	// may not follow column order) for the aggregated tables.
	clickhousePrimaryKeys []ColumnKey
}

// Column represents a column of data.
type Column struct {
	Key       ColumnKey
	Name      string
	Disabled  bool
	NoDisable bool
	Group     ColumnGroup
	Depends   []ColumnKey

	// For parser.
	ParserType string

	// For ClickHouse. `NotSortingKey' is for columns generated from other
	// columns. It is only useful if not ClickHouseMainOnly and not Alias.
	// `GenerateFrom' is for a column that's generated from an SQL expression
	// instead of being retrieved from the protobuf. `TransformFrom' and
	// `TransformTo' work in pairs. The first one is the set of column in the
	// raw table while the second one is how to transform it for the main table.
	ClickHouseType             string // ClickHouse type for the column
	ClickHouseMaterializedType string // ClickHouse type when we request materialization
	ClickHouseCodec            string // Compression codec
	ClickHouseAlias            string // Alias expression
	// ClickHouseNotSortingKey is to be used for columns whose content is
	// derived from another column. Like Exporter* all derive from
	// ExporterAddress.
	ClickHouseNotSortingKey bool
	// ClickHouseGenerateFrom computes the content of the column using another column
	ClickHouseGenerateFrom  string
	ClickHouseMainOnly      bool // Only include this column in the main table
	ClickHouseSelfGenerated bool // Generated (partly) from its own value
	// ClickHouseMaterialized indicates that the column was materialized (and is not by default)
	ClickHouseMaterialized bool

	// For the console. `ClickHouseTruncateIP' makes the specified column
	// truncatable when used as a dimension.
	ConsoleNotDimension bool
	ConsoleTruncateIP   bool
}

// ColumnKey is the name of a column
type ColumnKey uint

// ColumnGroup represents a group of columns
type ColumnGroup uint

// UInt128 is an unsigned 128-bit number
type UInt128 = proto.UInt128
