// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net/netip"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// This file implements an optional, append-time Protobuf encoding of enriched
// flows. It mirrors the pre-2.0 encoder (removed when the inlet/outlet split
// switched to direct columnar ClickHouse insertion) but writes into a per-flow
// buffer in parallel with the columnar batch. It is only exercised when the
// outlet Kafka output is enabled (see FlowMessage.EnableProtobuf); otherwise
// none of these methods are called and the ClickHouse-only path is unchanged.
//
// Each column is appended at most once per flow because the Append* helpers in
// clickhouse.go guard on columnSet before calling into here, so no separate
// "already set" tracking is needed.

// protobufAppendUint appends an integer column as a varint.
func (bf *FlowMessage) protobufAppendUint(columnKey ColumnKey, value uint64) {
	column := bf.protobufColumn(columnKey)
	if column == nil {
		return
	}
	bf.batch.protobuf = protowire.AppendTag(bf.batch.protobuf, column.ProtobufIndex, protowire.VarintType)
	bf.batch.protobuf = protowire.AppendVarint(bf.batch.protobuf, value)
}

// protobufAppendString appends a string column as a length-delimited field.
func (bf *FlowMessage) protobufAppendString(columnKey ColumnKey, value string) {
	column := bf.protobufColumn(columnKey)
	if column == nil {
		return
	}
	bf.batch.protobuf = protowire.AppendTag(bf.batch.protobuf, column.ProtobufIndex, protowire.BytesType)
	bf.batch.protobuf = protowire.AppendString(bf.batch.protobuf, value)
}

// protobufAppendIP appends an IP column as 16 length-delimited bytes (IPv6
// representation, matching the ClickHouse IPv6 columns).
func (bf *FlowMessage) protobufAppendIP(columnKey ColumnKey, value netip.Addr) {
	column := bf.protobufColumn(columnKey)
	if column == nil {
		return
	}
	v := value.As16()
	bf.batch.protobuf = protowire.AppendTag(bf.batch.protobuf, column.ProtobufIndex, protowire.BytesType)
	bf.batch.protobuf = protowire.AppendBytes(bf.batch.protobuf, v[:])
}

// protobufAppendArrayUint32 appends an Array(UInt32) column as a repeated
// (non-packed) varint field.
func (bf *FlowMessage) protobufAppendArrayUint32(columnKey ColumnKey, values []uint32) {
	column := bf.protobufColumn(columnKey)
	if column == nil {
		return
	}
	for _, v := range values {
		bf.batch.protobuf = protowire.AppendTag(bf.batch.protobuf, column.ProtobufIndex, protowire.VarintType)
		bf.batch.protobuf = protowire.AppendVarint(bf.batch.protobuf, uint64(v))
	}
}

// protobufAppendArrayUint128 appends an Array(UInt128) column as a repeated
// length-delimited field, each element being 16 bytes (high then low,
// big-endian).
func (bf *FlowMessage) protobufAppendArrayUint128(columnKey ColumnKey, values []UInt128) {
	column := bf.protobufColumn(columnKey)
	if column == nil {
		return
	}
	var buf [16]byte
	for _, v := range values {
		binary.BigEndian.PutUint64(buf[0:8], v.High)
		binary.BigEndian.PutUint64(buf[8:16], v.Low)
		bf.batch.protobuf = protowire.AppendTag(bf.batch.protobuf, column.ProtobufIndex, protowire.BytesType)
		bf.batch.protobuf = protowire.AppendBytes(bf.batch.protobuf, buf[:])
	}
}

// protobufColumn returns the column to encode for the given (already reversed)
// key, or nil when the column is not exported or Protobuf encoding is off.
func (bf *FlowMessage) protobufColumn(columnKey ColumnKey) *Column {
	if !bf.batch.protobufEnabled || int(columnKey) >= len(bf.schema.columnIndex) {
		return nil
	}
	column := bf.schema.columnIndex[columnKey]
	if column == nil || column.ProtobufIndex <= 0 {
		return nil
	}
	return column
}

// protobufFinalize captures the fully built flow into protobufMessage. It is
// called from Finalize() after the fixed struct fields have been appended and
// before the working buffer is reset.
//
// The working buffer (bf.batch.protobuf) is reused across flows, but the
// captured message is handed to the Kafka producer, which serializes it
// asynchronously and does not copy it. We therefore allocate a fresh slice per
// flow so the next flow's encoding cannot overwrite bytes still in flight.
func (bf *FlowMessage) protobufFinalize() {
	if !bf.batch.protobufEnabled {
		return
	}
	bf.batch.protobufMessage = append([]byte(nil), bf.batch.protobuf...)
}

// ProtobufDefinition returns the .proto definition matching the Protobuf
// encoding produced for the current schema.
//
// A few wire conventions are not captured by the proto3 field types alone and
// consumers need to know them:
//   - IP columns (IPv6 / LowCardinality(IPv6)) are 16-byte values, in IPv6
//     representation (IPv4 addresses are mapped into IPv6).
//   - Enum8 columns are encoded as their numeric value (uint32), not as named
//     proto enums.
//   - Array(UInt128) elements are 16-byte values: high 64 bits then low 64
//     bits, big-endian.
func (schema Schema) ProtobufDefinition() string {
	_, definition := schema.protobufMessageHashAndDefinition()
	return definition
}

// ProtobufMessageHash returns the hash identifying the current protobuf message
// layout. It is the same hash embedded in the generated message name
// (FlowMessagev<hash>), so it changes only when the wire layout changes.
func (schema Schema) ProtobufMessageHash() string {
	hash, _ := schema.protobufMessageHashAndDefinition()
	return hash
}

func (schema Schema) protobufMessageHashAndDefinition() (string, string) {
	lines := []string{}
	hash := fnv.New128()
	for _, column := range schema.Columns() {
		if column.ProtobufIndex <= 0 {
			continue
		}
		t := protobufTypeName(column.ProtobufType)
		if t == "" {
			continue
		}
		if column.ProtobufRepeated {
			t = fmt.Sprintf("repeated %s", t)
		}
		line := fmt.Sprintf("%s %s = %d;", t, column.Name, column.ProtobufIndex)
		lines = append(lines, line)
		hash.Write([]byte(line))
	}
	hashString := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash.Sum(nil))
	return hashString, fmt.Sprintf(`
syntax = "proto3";

message FlowMessagev%s {
 %s
}
`, hashString, strings.Join(lines, "\n "))
}

func protobufTypeName(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.Uint64Kind:
		return "uint64"
	case protoreflect.Uint32Kind:
		return "uint32"
	case protoreflect.BytesKind:
		return "bytes"
	default:
		return ""
	}
}
