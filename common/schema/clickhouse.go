// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/ch-go/proto"
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

// newProtoColumn turns a column into its proto.Column definition
func (column Column) newProtoColumn() proto.Column {
	if strings.HasPrefix(column.ClickHouseType, "Enum8(") {
		// Enum8 is a special case. We do not want to use ColAuto as it comes
		// with a performance penalty due to conversion between key values.
		return new(proto.ColEnum8)
	}

	col := &proto.ColAuto{}
	err := col.Infer(proto.ColumnType(column.ClickHouseType))
	if err != nil {
		panic(fmt.Sprintf("unhandled ClickHouse type %q", column.ClickHouseType))
	}
	return col.Data
}

// wrapProtoColumn optionally wraps the proto.Column for use in proto.Input
func (column Column) wrapProtoColumn(in proto.Column) proto.Column {
	if strings.HasPrefix(column.ClickHouseType, "Enum8(") {
		// Enum8 is a special case. See above.
		ddl := column.ClickHouseType[6 : len(column.ClickHouseType)-1]
		return proto.Wrap(in, ddl)
	}

	return in
}

// ClickHouseTableOption is an option to alter the values returned by ClickHouseCreateTable() and ClickHouseSelectColumns().
type ClickHouseTableOption int

const (
	// ClickHouseSkipMainOnlyColumns skips the columns for the main flows table only.
	ClickHouseSkipMainOnlyColumns ClickHouseTableOption = iota
	// ClickHouseSkipGeneratedColumns skips the columns with a GenerateFrom value
	ClickHouseSkipGeneratedColumns
	// ClickHouseSkipAliasedColumns skips the columns with a Alias value
	ClickHouseSkipAliasedColumns
	// ClickHouseSkipTimeReceived skips the time received column
	ClickHouseSkipTimeReceived
	// ClickHouseSubstituteGenerates changes the column name to use the default generated value
	ClickHouseSubstituteGenerates
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
		if slices.Contains(options, ClickHouseSkipGeneratedColumns) && column.ClickHouseGenerateFrom != "" && !column.ClickHouseSelfGenerated {
			continue
		}
		if slices.Contains(options, ClickHouseSkipAliasedColumns) && column.ClickHouseAlias != "" {
			continue
		}
		if slices.Contains(options, ClickHouseSubstituteGenerates) && column.ClickHouseGenerateFrom != "" {
			column.Name = fmt.Sprintf("%s AS %s", column.ClickHouseGenerateFrom, column.Name)
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
	for _, key := range schema.clickhousePrimaryKeys {
		cols = append(cols, key.String())
	}
	return cols
}

// ClickHouseHash returns an hash of the inpt table in ClickHouse
func (schema Schema) ClickHouseHash() string {
	hash := fnv.New128()
	create := schema.ClickHouseCreateTable(ClickHouseSkipGeneratedColumns, ClickHouseSkipAliasedColumns)
	hash.Write([]byte(create))
	hashString := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash.Sum(nil))
	return fmt.Sprintf("%sv5", hashString)
}

// AppendDateTime adds a DateTime value to the provided column
func (bf *FlowMessage) AppendDateTime(columnKey ColumnKey, value uint32) {
	col := bf.batch.columns[columnKey]
	if value == 0 || col == nil || bf.batch.columnSet.Test(uint(columnKey)) {
		return
	}
	bf.batch.columnSet.Set(uint(columnKey))
	col.(*proto.ColDateTime).AppendRaw(proto.DateTime(value))
	bf.appendDebug(columnKey, value)
}

// AppendUint adds an UInt64/32/16/8 or Enum8 value to the provided column
func (bf *FlowMessage) AppendUint(columnKey ColumnKey, value uint64) {
	col := bf.batch.columns[columnKey]
	if value == 0 || col == nil || bf.batch.columnSet.Test(uint(columnKey)) {
		return
	}
	switch col := col.(type) {
	case *proto.ColUInt64:
		col.Append(value)
	case *proto.ColUInt32:
		col.Append(uint32(value))
	case *proto.ColUInt16:
		col.Append(uint16(value))
	case *proto.ColUInt8:
		col.Append(uint8(value))
	case *proto.ColEnum8:
		col.Append(proto.Enum8(value))
	default:
		panic(fmt.Sprintf("unhandled uint type %q", col.Type()))
	}
	bf.batch.columnSet.Set(uint(columnKey))
	bf.appendDebug(columnKey, value)
}

// AppendString adds a String value to the provided column
func (bf *FlowMessage) AppendString(columnKey ColumnKey, value string) {
	col := bf.batch.columns[columnKey]
	if value == "" || col == nil || bf.batch.columnSet.Test(uint(columnKey)) {
		return
	}
	switch col := col.(type) {
	case *proto.ColLowCardinality[string]:
		col.Append(value)
	default:
		panic(fmt.Sprintf("unhandled string type %q", col.Type()))
	}
	bf.batch.columnSet.Set(uint(columnKey))
	bf.appendDebug(columnKey, value)
}

// AppendIPv6 adds an IPv6 value to the provided column
func (bf *FlowMessage) AppendIPv6(columnKey ColumnKey, value netip.Addr) {
	col := bf.batch.columns[columnKey]
	if !value.IsValid() || col == nil || bf.batch.columnSet.Test(uint(columnKey)) {
		return
	}
	switch col := col.(type) {
	case *proto.ColIPv6:
		col.Append(value.As16())
	case *proto.ColLowCardinality[proto.IPv6]:
		col.Append(value.As16())
	default:
		panic(fmt.Sprintf("unhandled string type %q", col.Type()))
	}
	bf.batch.columnSet.Set(uint(columnKey))
	bf.appendDebug(columnKey, value)
}

// AppendArrayUInt32 adds an Array(UInt32) value to the provided column
func (bf *FlowMessage) AppendArrayUInt32(columnKey ColumnKey, value []uint32) {
	col := bf.batch.columns[columnKey]
	if len(value) == 0 || col == nil || bf.batch.columnSet.Test(uint(columnKey)) {
		return
	}
	bf.batch.columnSet.Set(uint(columnKey))
	col.(*proto.ColArr[uint32]).Append(value)
	bf.appendDebug(columnKey, value)
}

// AppendArrayUInt128 adds an Array(UInt128) value to the provided column
func (bf *FlowMessage) AppendArrayUInt128(columnKey ColumnKey, value []UInt128) {
	col := bf.batch.columns[columnKey]
	if len(value) == 0 || col == nil || bf.batch.columnSet.Test(uint(columnKey)) {
		return
	}
	bf.batch.columnSet.Set(uint(columnKey))
	col.(*proto.ColArr[proto.UInt128]).Append(value)
	bf.appendDebug(columnKey, value)
}

func (bf *FlowMessage) appendDebug(columnKey ColumnKey, value any) {
	if !debug {
		return
	}
	if bf.OtherColumns == nil {
		bf.OtherColumns = make(map[ColumnKey]any)
	}
	bf.OtherColumns[columnKey] = value
}

// check executes some sanity checks when in debug mode. It should be called
// only after finalization.
func (bf *FlowMessage) check() {
	if !debug {
		return
	}
	if debug {
		// Check that all columns have the right amount of rows
		for idx, col := range bf.batch.columns {
			if col == nil {
				continue
			}
			if col.Rows() != bf.batch.rowCount {
				panic(fmt.Sprintf("row %s has a count of %d instead of %d", ColumnKey(idx), col.Rows(), bf.batch.rowCount))
			}
		}
	}
}

// appendDefaultValue appends a default/zero value to the given column.
func (bf *FlowMessage) appendDefaultValues() {
	for idx, col := range bf.batch.columns {
		// Skip unpopulated columns
		if col == nil {
			continue
		}
		// Or columns already set
		if bf.batch.columnSet.Test(uint(idx)) {
			continue
		}
		// Put the default value depending on the real type
		switch col := col.(type) {
		case *proto.ColUInt64:
			col.Append(0)
		case *proto.ColUInt32:
			col.Append(0)
		case *proto.ColUInt16:
			col.Append(0)
		case *proto.ColUInt8:
			col.Append(0)
		case *proto.ColIPv6:
			col.Append([16]byte{})
		case *proto.ColDateTime:
			col.Append(time.Unix(0, 0))
		case *proto.ColEnum8:
			col.Append(0)
		case *proto.ColLowCardinality[string]:
			col.Append("")
		case *proto.ColLowCardinality[proto.IPv6]:
			col.Append(proto.IPv6{})
		case *proto.ColArr[uint32]:
			col.Append([]uint32{})
		case *proto.ColArr[proto.UInt128]:
			col.Append([]proto.UInt128{})
		default:
			panic(fmt.Sprintf("unhandled ClickHouse type %q", col.Type()))
		}
	}
}

// Undo reverts the current changes. This should revert the various Append() functions.
func (bf *FlowMessage) Undo() {
	for idx, col := range bf.batch.columns {
		if col == nil {
			continue
		}
		if !bf.batch.columnSet.Test(uint(idx)) {
			continue
		}
		switch col := col.(type) {
		case *proto.ColUInt64:
			*col = (*col)[:len(*col)-1]
		case *proto.ColUInt32:
			*col = (*col)[:len(*col)-1]
		case *proto.ColUInt16:
			*col = (*col)[:len(*col)-1]
		case *proto.ColUInt8:
			*col = (*col)[:len(*col)-1]
		case *proto.ColIPv6:
			*col = (*col)[:len(*col)-1]
		case *proto.ColDateTime:
			col.Data = col.Data[:len(col.Data)-1]
		case *proto.ColEnum8:
			*col = (*col)[:len(*col)-1]
		case *proto.ColLowCardinality[string]:
			col.Values = col.Values[:len(col.Values)-1]
		case *proto.ColLowCardinality[proto.IPv6]:
			col.Values = col.Values[:len(col.Values)-1]
		case *proto.ColArr[uint32]:
			l := len(col.Offsets)
			if l > 0 {
				start := uint64(0)
				if l > 1 {
					start = col.Offsets[l-2]
				}
				data := col.Data.(*proto.ColUInt32)
				*data = (*data)[:start]
				col.Data = data
				col.Offsets = col.Offsets[:l-1]
			}
		case *proto.ColArr[proto.UInt128]:
			l := len(col.Offsets)
			if l > 0 {
				start := uint64(0)
				if l > 1 {
					start = col.Offsets[l-2]
				}
				data := col.Data.(*proto.ColUInt128)
				*data = (*data)[:start]
				col.Data = data
				col.Offsets = col.Offsets[:l-1]
			}
		default:
			panic(fmt.Sprintf("unhandled ClickHouse type %q", col.Type()))
		}
	}
	bf.reset()
}

// Finalize finalizes the current FlowMessage. It can then be reused for the
// next one. It is crucial to always call Finalize, otherwise the batch could be
// faulty.
func (bf *FlowMessage) Finalize() {
	bf.AppendDateTime(ColumnTimeReceived, bf.TimeReceived)
	bf.AppendUint(ColumnSamplingRate, bf.SamplingRate)
	bf.AppendIPv6(ColumnExporterAddress, bf.ExporterAddress)
	bf.AppendUint(ColumnSrcAS, uint64(bf.SrcAS))
	bf.AppendUint(ColumnDstAS, uint64(bf.DstAS))
	bf.AppendUint(ColumnSrcNetMask, uint64(bf.SrcNetMask))
	bf.AppendUint(ColumnDstNetMask, uint64(bf.DstNetMask))
	bf.AppendIPv6(ColumnSrcAddr, bf.SrcAddr)
	bf.AppendIPv6(ColumnDstAddr, bf.DstAddr)
	bf.AppendIPv6(ColumnNextHop, bf.NextHop)
	if !bf.schema.IsDisabled(ColumnGroupL2) {
		bf.AppendUint(ColumnSrcVlan, uint64(bf.SrcVlan))
		bf.AppendUint(ColumnDstVlan, uint64(bf.DstVlan))
	}
	bf.batch.rowCount++
	bf.appendDefaultValues()
	bf.reset()
	bf.check()
}
