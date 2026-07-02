// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"net/netip"

	"github.com/ClickHouse/ch-go/proto"
	"github.com/bits-and-blooms/bitset"
)

// FlowMessage is the abstract representation of a flow through various subsystems.
type FlowMessage struct {
	TimeReceived uint32
	SamplingRate uint64

	// For exporter classifier
	ExporterAddress netip.Addr

	// For interface classifier
	InIf    uint32
	OutIf   uint32
	SrcVlan uint16
	DstVlan uint16

	// For routing component
	SrcAddr netip.Addr
	DstAddr netip.Addr
	NextHop netip.Addr

	// Core component may override them
	SrcAS      uint32
	DstAS      uint32
	SrcNetMask uint8
	DstNetMask uint8

	// Only for tests
	OtherColumns map[ColumnKey]any

	reversed bool
	batch    clickhouseBatch
	schema   *Schema
}

// clickhouseBatch stores columns for efficient streaming. It is embedded
// inside a FlowMessage.
type clickhouseBatch struct {
	columns   []proto.Column // Indexed by ColumnKey
	columnSet bitset.BitSet  // Track which columns have been set
	rowCount  int            // Number of rows accumulated
	input     proto.Input    // Input including all columns to stream to ClickHouse

	// Optional Protobuf encoding of the enriched flow, populated in parallel
	// with the columnar batch when the outlet Kafka output is enabled. When
	// disabled (the default), none of this is touched.
	protobufEnabled bool   // Whether to encode flows to Protobuf
	protobuf        []byte // Working buffer for the flow currently being built
	protobufMessage []byte // Last finalized flow, valid until the next Finalize
}

// reset resets a flow message. All public fields are set to 0,
// but the current ClickHouse batch is left untouched.
func (bf *FlowMessage) reset() {
	*bf = FlowMessage{
		batch:  bf.batch,
		schema: bf.schema,
	}
	bf.batch.columnSet.ClearAll()
	// Discard the in-progress Protobuf flow (the finalized message, if any, is
	// kept in protobufMessage for the caller to read).
	bf.batch.protobuf = bf.batch.protobuf[:0]
}

// Clear clears all column data. The last finalized Protobuf message
// (protobufMessage) is intentionally left intact: it is overwritten on the next
// Finalize and the caller reads it right after FinalizeAndSend, which may flush
// (and thus Clear) the batch as part of the same call.
func (bf *FlowMessage) Clear() {
	bf.reset()
	bf.batch.input.Reset()
	bf.batch.rowCount = 0
}

// EnableProtobuf turns on Protobuf encoding of enriched flows for this message.
// It must be called before the message is used. When off (the default), the
// ClickHouse columnar path is entirely unaffected.
func (bf *FlowMessage) EnableProtobuf() {
	bf.batch.protobufEnabled = true
	if bf.batch.protobuf == nil {
		bf.batch.protobuf = make([]byte, 0, 512)
	}
}

// ProtobufMessage returns the Protobuf encoding of the last finalized flow. It
// is only valid right after Finalize() and until the next Finalize(). Returns
// nil when Protobuf encoding is disabled.
func (bf *FlowMessage) ProtobufMessage() []byte {
	return bf.batch.protobufMessage
}

// ClickHouseProtoInput returns the proto.Input that can be used to stream results
// to ClickHouse.
func (bf *FlowMessage) ClickHouseProtoInput() proto.Input {
	return bf.batch.input
}

// NewFlowMessage creates a new FlowMessage for the given schema with ClickHouse batch initialized.
func (schema *Schema) NewFlowMessage() *FlowMessage {
	bf := &FlowMessage{
		schema: schema,
	}

	maxKey := ColumnKey(0)
	for _, column := range bf.schema.columns {
		if column.Key > maxKey {
			maxKey = column.Key
		}
	}

	bf.batch.columns = make([]proto.Column, maxKey+1)
	bf.batch.columnSet = *bitset.New(uint(maxKey + 1))
	bf.batch.rowCount = 0

	for _, column := range bf.schema.columns {
		if !column.Disabled && column.shouldProvideValue() {
			bf.batch.columns[column.Key] = column.newProtoColumn()
			bf.batch.input = append(bf.batch.input, proto.InputColumn{
				Name: column.Name,
				Data: column.wrapProtoColumn(bf.batch.columns[column.Key]),
			})
		}
	}

	return bf
}

// FlowCount return the number of flows batched
func (bf *FlowMessage) FlowCount() int {
	return bf.batch.rowCount
}

// Reverse reverses the direction of the next calls to Append*().
func (bf *FlowMessage) Reverse() {
	bf.reversed = !bf.reversed
	bf.InIf, bf.OutIf = bf.OutIf, bf.InIf
	bf.SrcVlan, bf.DstVlan = bf.DstVlan, bf.SrcVlan
	bf.SrcAddr, bf.DstAddr = bf.DstAddr, bf.SrcAddr
	bf.SrcAS, bf.DstAS = bf.DstAS, bf.SrcAS
	bf.SrcNetMask, bf.DstNetMask = bf.DstNetMask, bf.SrcNetMask
}
