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

	batch  clickhouseBatch
	schema *Schema
}

// clickhouseBatch stores columns for efficient streaming. It is embedded
// inside a FlowMessage.
type clickhouseBatch struct {
	columns   []proto.Column // Indexed by ColumnKey
	columnSet bitset.BitSet  // Track which columns have been set
	rowCount  int            // Number of rows accumulated
	input     proto.Input    // Input including all columns to stream to ClickHouse
}

// reset resets a flow message. All public fields are set to 0,
// but the current ClickHouse batch is left untouched.
func (bf *FlowMessage) reset() {
	*bf = FlowMessage{
		batch:  bf.batch,
		schema: bf.schema,
	}
	bf.batch.columnSet.ClearAll()
}

// Clear clears all column data.
func (bf *FlowMessage) Clear() {
	bf.reset()
	bf.batch.input.Reset()
	bf.batch.rowCount = 0
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
