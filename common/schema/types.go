// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"net/netip"

	"github.com/bits-and-blooms/bitset"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Schema is the data schema.
type Schema struct {
	columns     []Column  // Ordered list of columns
	columnIndex []*Column // Columns indexed by ColumnKey

	// For ClickHouse. This is the set of primary keys (order is important and
	// may not follow column order).
	clickHousePrimaryKeys []ColumnKey
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

	// For protobuf. The index is automatically derived from the position,
	// unless specified. Use -1 to not include the column into the protobuf
	// schema.
	ProtobufIndex    protowire.Number
	ProtobufType     protoreflect.Kind // Uint64Kind, Uint32Kind, BytesKind, StringKind, EnumKind
	ProtobufEnum     map[int]string
	ProtobufEnumName string
	ProtobufRepeated bool
}

// ColumnKey is the name of a column
type ColumnKey int

// FlowMessage is the abstract representation of a flow through various subsystems.
type FlowMessage struct {
	TimeReceived uint64
	SamplingRate uint32

	// For exporter classifier
	ExporterAddress netip.Addr

	// For interface classifier
	InIf  uint32
	OutIf uint32

	// For geolocation or BMP
	SrcAddr netip.Addr
	DstAddr netip.Addr
	NextHop netip.Addr

	// Core component may override them
	SrcAS uint32
	DstAS uint32

	// protobuf is the protobuf representation for the information not contained above.
	protobuf      []byte
	protobufSet   bitset.BitSet
	ProtobufDebug map[ColumnKey]interface{} `json:"-"` // for testing purpose
}

const maxSizeVarint = 10 // protowire.SizeVarint(^uint64(0))
