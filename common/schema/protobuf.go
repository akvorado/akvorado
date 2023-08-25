// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"net/netip"
	"strings"

	"github.com/bits-and-blooms/bitset"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ProtobufMessageHash returns the name of the protobuf definition.
func (schema Schema) ProtobufMessageHash() string {
	name, _ := schema.protobufMessageHashAndDefinition()
	return name
}

// ProtobufDefinition returns the protobuf definition.
func (schema Schema) ProtobufDefinition() string {
	_, definition := schema.protobufMessageHashAndDefinition()
	return definition
}

// protobufMessageHashAndDefinition returns the name of the protobuf definition
// along with the protobuf definition itself (.proto file).
func (schema Schema) protobufMessageHashAndDefinition() (string, string) {
	lines := []string{}
	enums := map[string]string{}

	hash := fnv.New128()
	for _, column := range schema.Columns() {
		for _, column := range append([]Column{column}, column.ClickHouseTransformFrom...) {
			if column.ProtobufIndex < 0 {
				continue
			}

			t := column.ProtobufType.String()

			// Enum definition
			if column.ProtobufType == protoreflect.EnumKind {
				if _, ok := enums[column.ProtobufEnumName]; !ok {
					definition := []string{}
					keys := []int{}
					for key := range column.ProtobufEnum {
						keys = append(keys, key)
					}
					slices.Sort(keys)
					for _, key := range keys {
						definition = append(definition, fmt.Sprintf("%s = %d;", column.ProtobufEnum[key], key))
					}
					enums[column.ProtobufEnumName] = fmt.Sprintf("enum %s { %s }",
						column.ProtobufEnumName,
						strings.Join(definition, " "))
				}
				t = column.ProtobufEnumName
			}

			// Column definition
			if column.ProtobufRepeated {
				t = fmt.Sprintf("repeated %s", t)
			}
			line := fmt.Sprintf("%s %s = %d;",
				t,
				column.Name,
				column.ProtobufIndex,
			)
			lines = append(lines, line)
			hash.Write([]byte(line))
		}
	}

	enumDefinitions := []string{}
	for _, v := range enums {
		enumDefinitions = append(enumDefinitions, v)
		hash.Write([]byte(v))
	}
	hashString := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash.Sum(nil))

	return hashString, fmt.Sprintf(`
syntax = "proto3";

message FlowMessagev%s {
 %s

 %s
}
`, hashString, strings.Join(enumDefinitions, "\n "), strings.Join(lines, "\n "))
}

// ProtobufMarshal transforms a basic flow into protobuf bytes. The provided flow should
// not be reused afterwards.
func (schema *Schema) ProtobufMarshal(bf *FlowMessage) []byte {
	schema.ProtobufAppendVarint(bf, ColumnTimeReceived, bf.TimeReceived)
	schema.ProtobufAppendVarint(bf, ColumnSamplingRate, uint64(bf.SamplingRate))
	schema.ProtobufAppendIP(bf, ColumnExporterAddress, bf.ExporterAddress)
	schema.ProtobufAppendVarint(bf, ColumnSrcAS, uint64(bf.SrcAS))
	schema.ProtobufAppendVarint(bf, ColumnDstAS, uint64(bf.DstAS))
	schema.ProtobufAppendVarint(bf, ColumnSrcNetMask, uint64(bf.SrcNetMask))
	schema.ProtobufAppendVarint(bf, ColumnDstNetMask, uint64(bf.DstNetMask))
	schema.ProtobufAppendIP(bf, ColumnSrcAddr, bf.SrcAddr)
	schema.ProtobufAppendIP(bf, ColumnDstAddr, bf.DstAddr)
	schema.ProtobufAppendIP(bf, ColumnNextHop, bf.NextHop)
	if !schema.IsDisabled(ColumnGroupL2) {
		schema.ProtobufAppendVarint(bf, ColumnSrcVlan, uint64(bf.SrcVlan))
		schema.ProtobufAppendVarint(bf, ColumnDstVlan, uint64(bf.DstVlan))
	}

	// Add length and move it as a prefix
	end := len(bf.protobuf)
	payloadLen := end - maxSizeVarint
	bf.protobuf = protowire.AppendVarint(bf.protobuf, uint64(payloadLen))
	sizeLen := len(bf.protobuf) - end
	result := bf.protobuf[maxSizeVarint-sizeLen : end]
	copy(result, bf.protobuf[end:end+sizeLen])

	return result
}

// ProtobufAppendVarint append a varint to the protobuf representation of a flow.
func (schema *Schema) ProtobufAppendVarint(bf *FlowMessage, columnKey ColumnKey, value uint64) {
	// Check if value is 0 to avoid a lookup.
	if value > 0 {
		schema.ProtobufAppendVarintForce(bf, columnKey, value)
	}
}

// ProtobufAppendVarintForce append a varint to the protobuf representation of a flow, even if it is a 0-value.
func (schema *Schema) ProtobufAppendVarintForce(bf *FlowMessage, columnKey ColumnKey, value uint64) {
	column, _ := schema.LookupColumnByKey(columnKey)
	column.ProtobufAppendVarintForce(bf, value)
}

// ProtobufAppendVarint append a varint to the protobuf representation of a flow.
func (column *Column) ProtobufAppendVarint(bf *FlowMessage, value uint64) {
	if value > 0 {
		column.ProtobufAppendVarintForce(bf, value)
	}
}

// ProtobufAppendVarintForce append a varint to the protobuf representation of a flow, even when 0.
func (column *Column) ProtobufAppendVarintForce(bf *FlowMessage, value uint64) {
	bf.init()
	if column.protobufCanAppend(bf) {
		bf.protobuf = protowire.AppendTag(bf.protobuf, column.ProtobufIndex, protowire.VarintType)
		bf.protobuf = protowire.AppendVarint(bf.protobuf, value)
		bf.protobufSet.Set(uint(column.ProtobufIndex))
		if debug {
			column.appendDebug(bf, value)
		}
	}
}

func (column Column) protobufCanAppend(bf *FlowMessage) bool {
	return column.ProtobufIndex > 0 &&
		!column.Disabled &&
		(column.ProtobufRepeated || !bf.protobufSet.Test(uint(column.ProtobufIndex)))
}

// ProtobufAppendBytes append a slice of bytes to the protobuf representation
// of a flow.
func (schema *Schema) ProtobufAppendBytes(bf *FlowMessage, columnKey ColumnKey, value []byte) {
	if len(value) > 0 {
		schema.ProtobufAppendBytesForce(bf, columnKey, value)
	}
}

// ProtobufAppendBytesForce append a slice of bytes to the protobuf representation
// of a flow, even when empty
func (schema *Schema) ProtobufAppendBytesForce(bf *FlowMessage, columnKey ColumnKey, value []byte) {
	column, _ := schema.LookupColumnByKey(columnKey)
	column.ProtobufAppendBytesForce(bf, value)
}

// ProtobufAppendBytes append a slice of bytes to the protobuf representation
// of a flow.
func (column *Column) ProtobufAppendBytes(bf *FlowMessage, value []byte) {
	if len(value) > 0 {
		column.ProtobufAppendBytesForce(bf, value)
	}
}

// ProtobufAppendBytesForce append a slice of bytes to the protobuf representation
// of a flow, even when empty
func (column *Column) ProtobufAppendBytesForce(bf *FlowMessage, value []byte) {
	bf.init()
	if column.protobufCanAppend(bf) {
		bf.protobuf = protowire.AppendTag(bf.protobuf, column.ProtobufIndex, protowire.BytesType)
		bf.protobuf = protowire.AppendBytes(bf.protobuf, value)
		bf.protobufSet.Set(uint(column.ProtobufIndex))
		if debug {
			column.appendDebug(bf, value)
		}
	}
}

// ProtobufAppendIP append an IP to the protobuf representation
// of a flow.
func (schema *Schema) ProtobufAppendIP(bf *FlowMessage, columnKey ColumnKey, value netip.Addr) {
	if value.IsValid() {
		column, _ := schema.LookupColumnByKey(columnKey)
		column.ProtobufAppendIPForce(bf, value)
	}
}

// ProtobufAppendIP append an IP to the protobuf representation
// of a flow.
func (column *Column) ProtobufAppendIP(bf *FlowMessage, value netip.Addr) {
	if value.IsValid() {
		column.ProtobufAppendIPForce(bf, value)
	}
}

// ProtobufAppendIPForce append an IP to the protobuf representation
// of a flow, even when not valid
func (column *Column) ProtobufAppendIPForce(bf *FlowMessage, value netip.Addr) {
	bf.init()
	if column.protobufCanAppend(bf) {
		v := value.As16()
		bf.protobuf = protowire.AppendTag(bf.protobuf, column.ProtobufIndex, protowire.BytesType)
		bf.protobuf = protowire.AppendBytes(bf.protobuf, v[:])
		bf.protobufSet.Set(uint(column.ProtobufIndex))
		if debug {
			column.appendDebug(bf, value)
		}
	}
}

func (column *Column) appendDebug(bf *FlowMessage, value interface{}) {
	if bf.ProtobufDebug == nil {
		bf.ProtobufDebug = make(map[ColumnKey]interface{})
	}
	if column.ProtobufRepeated {
		if current, ok := bf.ProtobufDebug[column.Key]; ok {
			bf.ProtobufDebug[column.Key] = append(current.([]interface{}), value)
		} else {
			bf.ProtobufDebug[column.Key] = []interface{}{value}
		}
	} else {
		bf.ProtobufDebug[column.Key] = value
	}
}

func (bf *FlowMessage) init() {
	if bf.protobuf == nil {
		bf.protobuf = make([]byte, maxSizeVarint, 500)
		bf.protobufSet = *bitset.New(uint(ColumnLast))
	}
}
