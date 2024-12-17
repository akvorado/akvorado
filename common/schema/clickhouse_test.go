// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"net/netip"
	"slices"
	"testing"

	"akvorado/common/helpers"

	"github.com/ClickHouse/ch-go/proto"
)

func TestAppendDefault(t *testing.T) {
	c := NewMock(t).EnableAllColumns()
	bf := c.NewFlowMessage()
	bf.Finalize()
	if bf.batch.rowCount != 1 {
		t.Errorf("rowCount should be 1, not %d", bf.batch.rowCount)
	}
	if bf.batch.columnSet.Any() {
		t.Error("columnSet should be empty after finalize")
	}
	for idx, col := range bf.batch.columns {
		if col == nil {
			continue
		}
		if col.Rows() != 1 {
			t.Errorf("column %q should be length 1", ColumnKey(idx))
		}
	}
}

func TestAppendBasics(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Test basic append
	bf.AppendDateTime(ColumnTimeReceived, 1000)
	bf.AppendUint(ColumnSamplingRate, 20000)
	bf.AppendUint(ColumnDstAS, 65000)

	// Test zero value (should not append)
	bf.AppendUint(ColumnSrcAS, 0)

	// Test duplicate append
	bf.AppendUint(ColumnPackets, 100)
	bf.AppendUint(ColumnPackets, 200)

	expected := map[ColumnKey]any{
		ColumnTimeReceived: 1000,
		ColumnSamplingRate: 20000,
		ColumnDstAS:        65000,
		ColumnPackets:      100,
	}
	got := bf.OtherColumns
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Append() (-got, +want):\n%s", diff)
	}

	bf.Finalize()
	for idx, col := range bf.batch.columns {
		if col == nil {
			continue
		}
		if col.Rows() != 1 {
			t.Errorf("column %q should be length 1", ColumnKey(idx))
		}
	}
}

func TestAppendWithDisabledColumns(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Try to append to a disabled column (L2 group is disabled by default in mock)
	bf.AppendUint(ColumnSrcVlan, 100)
	bf.Finalize()
}

func TestAppendArrayUInt32Columns(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()
	bf.AppendArrayUInt32(ColumnDstASPath, []uint32{65400, 65500, 65001})
	bf.Finalize()
	bf.AppendArrayUInt32(ColumnDstASPath, []uint32{65403, 65503, 65003})
	bf.Finalize()

	// Verify column has data
	got := bf.batch.columns[ColumnDstASPath].(*proto.ColArr[uint32])
	expected := proto.ColArr[uint32]{
		Offsets: proto.ColUInt64{3, 6},
		Data:    &proto.ColUInt32{65400, 65500, 65001, 65403, 65503, 65003},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("AppendArrayUInt32 (-got, +want):\n%s", diff)
	}
}

func TestAppendArrayUInt128Columns(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()
	bf.AppendArrayUInt128(ColumnDstLargeCommunities, []UInt128{
		{
			High: (65401 << 32) + 100,
			Low:  200,
		},
		{
			High: (65401 << 32) + 100,
			Low:  201,
		},
	})
	bf.Finalize()

	got := bf.batch.columns[ColumnDstLargeCommunities].(*proto.ColArr[proto.UInt128])
	expected := proto.ColArr[proto.UInt128]{
		Offsets: proto.ColUInt64{2},
		Data: &proto.ColUInt128{
			{High: (65401 << 32) + 100, Low: 200},
			{High: (65401 << 32) + 100, Low: 201},
		},
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("AppendArrayUInt128 (-got, +want):\n%s", diff)
	}
}

func TestUndoUInt64(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add two values
	bf.AppendUint(ColumnBytes, 100)
	bf.AppendUint(ColumnPackets, 200)

	// Check we have the expected initial state
	bytesCol := bf.batch.columns[ColumnBytes].(*proto.ColUInt64)
	packetsCol := bf.batch.columns[ColumnPackets].(*proto.ColUInt64)

	expectedBytes := proto.ColUInt64{100}
	expectedPackets := proto.ColUInt64{200}

	if diff := helpers.Diff(bytesCol, expectedBytes); diff != "" {
		t.Errorf("Initial bytes column state (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(packetsCol, expectedPackets); diff != "" {
		t.Errorf("Initial packets column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the last appended values
	bf.Undo()

	expectedBytesAfter := proto.ColUInt64{}
	expectedPacketsAfter := proto.ColUInt64{}

	if diff := helpers.Diff(bytesCol, expectedBytesAfter); diff != "" {
		t.Errorf("Bytes column after undo (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(packetsCol, expectedPacketsAfter); diff != "" {
		t.Errorf("Packets column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoUInt32(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add two values
	bf.AppendUint(ColumnSrcAS, 65001)
	bf.AppendUint(ColumnDstAS, 65002)

	// Check we have the expected initial state
	srcCol := bf.batch.columns[ColumnSrcAS].(*proto.ColUInt32)
	dstCol := bf.batch.columns[ColumnDstAS].(*proto.ColUInt32)

	expectedSrc := proto.ColUInt32{65001}
	expectedDst := proto.ColUInt32{65002}

	if diff := helpers.Diff(srcCol, expectedSrc); diff != "" {
		t.Errorf("Initial SrcAS column state (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(dstCol, expectedDst); diff != "" {
		t.Errorf("Initial DstAS column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the last appended values
	bf.Undo()

	expectedSrcAfter := proto.ColUInt32{}
	expectedDstAfter := proto.ColUInt32{}

	if diff := helpers.Diff(srcCol, expectedSrcAfter); diff != "" {
		t.Errorf("SrcAS column after undo (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(dstCol, expectedDstAfter); diff != "" {
		t.Errorf("DstAS column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoUInt16(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add two values
	bf.AppendUint(ColumnSrcPort, 80)
	bf.AppendUint(ColumnDstPort, 443)

	// Check we have the expected initial state
	srcCol := bf.batch.columns[ColumnSrcPort].(*proto.ColUInt16)
	dstCol := bf.batch.columns[ColumnDstPort].(*proto.ColUInt16)

	expectedSrc := proto.ColUInt16{80}
	expectedDst := proto.ColUInt16{443}

	if diff := helpers.Diff(srcCol, expectedSrc); diff != "" {
		t.Errorf("Initial SrcPort column state (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(dstCol, expectedDst); diff != "" {
		t.Errorf("Initial DstPort column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the last appended values
	bf.Undo()

	expectedSrcAfter := proto.ColUInt16{}
	expectedDstAfter := proto.ColUInt16{}

	if diff := helpers.Diff(srcCol, expectedSrcAfter); diff != "" {
		t.Errorf("SrcPort column after undo (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(dstCol, expectedDstAfter); diff != "" {
		t.Errorf("DstPort column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoUInt8(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add value
	bf.AppendUint(ColumnSrcNetMask, 6)

	// Check we have the expected initial state
	col := bf.batch.columns[ColumnSrcNetMask].(*proto.ColUInt8)
	expected := proto.ColUInt8{6}

	if diff := helpers.Diff(col, expected); diff != "" {
		t.Errorf("Initial Proto column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the last appended value
	bf.Undo()

	expectedAfter := proto.ColUInt8{}

	if diff := helpers.Diff(col, expectedAfter); diff != "" {
		t.Errorf("Proto column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoIPv6(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add IPv6 values
	srcAddr := netip.MustParseAddr("2001:db8::1")
	dstAddr := netip.MustParseAddr("2001:db8::2")

	bf.AppendIPv6(ColumnSrcAddr, srcAddr)
	bf.AppendIPv6(ColumnDstAddr, dstAddr)

	// Check we have the expected initial state
	srcCol := bf.batch.columns[ColumnSrcAddr].(*proto.ColIPv6)
	dstCol := bf.batch.columns[ColumnDstAddr].(*proto.ColIPv6)

	expectedSrc := proto.ColIPv6{srcAddr.As16()}
	expectedDst := proto.ColIPv6{dstAddr.As16()}

	if diff := helpers.Diff(srcCol, expectedSrc); diff != "" {
		t.Errorf("Initial SrcAddr column state (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(dstCol, expectedDst); diff != "" {
		t.Errorf("Initial DstAddr column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the values
	bf.Undo()

	expectedSrcAfter := proto.ColIPv6{}
	expectedDstAfter := proto.ColIPv6{}

	if diff := helpers.Diff(srcCol, expectedSrcAfter); diff != "" {
		t.Errorf("SrcAddr column after undo (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(dstCol, expectedDstAfter); diff != "" {
		t.Errorf("DstAddr column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoDateTime(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add DateTime value
	bf.AppendDateTime(ColumnTimeReceived, 1000)

	// Check we have the expected initial state
	col := bf.batch.columns[ColumnTimeReceived].(*proto.ColDateTime)
	expected := proto.ColDateTime{Data: []proto.DateTime{1000}}

	if diff := helpers.Diff(col, expected); diff != "" {
		t.Errorf("Initial TimeReceived column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the value
	bf.Undo()

	expectedAfter := proto.ColDateTime{Data: []proto.DateTime{}}

	if diff := helpers.Diff(col, expectedAfter); diff != "" {
		t.Errorf("TimeReceived column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoEnum8(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add Enum8 value (using interface boundary enum)
	bf.AppendUint(ColumnInIfBoundary, uint64(InterfaceBoundaryExternal))

	// Check we have the expected initial state
	col := bf.batch.columns[ColumnInIfBoundary].(*proto.ColEnum8)
	expected := proto.ColEnum8{proto.Enum8(InterfaceBoundaryExternal)}

	if diff := helpers.Diff(col, expected); diff != "" {
		t.Errorf("Initial InIfBoundary column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the value
	bf.Undo()

	expectedAfter := proto.ColEnum8{}

	if diff := helpers.Diff(col, expectedAfter); diff != "" {
		t.Errorf("InIfBoundary column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoLowCardinalityString(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add LowCardinality string values
	bf.AppendString(ColumnExporterName, "router1")
	bf.AppendString(ColumnExporterRole, "edge")

	// Check we have the expected initial state
	nameCol := bf.batch.columns[ColumnExporterName].(*proto.ColLowCardinality[string])
	roleCol := bf.batch.columns[ColumnExporterRole].(*proto.ColLowCardinality[string])

	expectedName := proto.ColLowCardinality[string]{Values: []string{"router1"}}
	expectedRole := proto.ColLowCardinality[string]{Values: []string{"edge"}}

	if diff := helpers.Diff(nameCol, expectedName); diff != "" {
		t.Errorf("Initial ExporterName column state (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(roleCol, expectedRole); diff != "" {
		t.Errorf("Initial ExporterRole column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the values
	bf.Undo()

	expectedNameAfter := proto.ColLowCardinality[string]{Values: []string{}}
	expectedRoleAfter := proto.ColLowCardinality[string]{Values: []string{}}

	if diff := helpers.Diff(nameCol, expectedNameAfter); diff != "" {
		t.Errorf("ExporterName column after undo (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(roleCol, expectedRoleAfter); diff != "" {
		t.Errorf("ExporterRole column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoLowCardinalityIPv6(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()

	// Add LowCardinality IPv6 value
	addr := netip.MustParseAddr("2001:db8::1")
	bf.AppendIPv6(ColumnExporterAddress, addr)

	// Check we have the expected initial state
	col := bf.batch.columns[ColumnExporterAddress].(*proto.ColLowCardinality[proto.IPv6])
	expected := proto.ColLowCardinality[proto.IPv6]{Values: []proto.IPv6{addr.As16()}}

	if diff := helpers.Diff(col, expected); diff != "" {
		t.Errorf("Initial ExporterAddress column state (-got, +want):\n%s", diff)
	}

	// Undo should remove the value
	bf.Undo()

	expectedAfter := proto.ColLowCardinality[proto.IPv6]{Values: []proto.IPv6{}}

	if diff := helpers.Diff(col, expectedAfter); diff != "" {
		t.Errorf("ExporterAddress column after undo (-got, +want):\n%s", diff)
	}
}

func TestUndoArrayUInt32(t *testing.T) {
	c := NewMock(t)

	t.Run("one value", func(t *testing.T) {
		bf := c.NewFlowMessage()
		bf.AppendArrayUInt32(ColumnDstASPath, []uint32{65001, 65002, 65003})

		// Check we have the expected initial state
		col := bf.batch.columns[ColumnDstASPath].(*proto.ColArr[uint32])
		expected := proto.ColArr[uint32]{
			Offsets: proto.ColUInt64{3},
			Data:    &proto.ColUInt32{65001, 65002, 65003},
		}

		if diff := helpers.Diff(*col, expected); diff != "" {
			t.Errorf("Initial DstASPath column state (-got, +want):\n%s", diff)
		}

		// Undo should remove the array
		bf.Undo()

		expectedAfter := proto.ColArr[uint32]{
			Offsets: proto.ColUInt64{},
			Data:    &proto.ColUInt32{},
		}

		if diff := helpers.Diff(*col, expectedAfter); diff != "" {
			t.Errorf("DstASPath column after undo (-got, +want):\n%s", diff)
		}
	})

	t.Run("two values", func(t *testing.T) {
		bf := c.NewFlowMessage()
		bf.AppendArrayUInt32(ColumnDstASPath, []uint32{65001, 65002, 65003})
		bf.Finalize()
		bf.AppendArrayUInt32(ColumnDstASPath, []uint32{65007, 65008})

		// Check we have the expected initial state
		col := bf.batch.columns[ColumnDstASPath].(*proto.ColArr[uint32])
		expected := proto.ColArr[uint32]{
			Offsets: proto.ColUInt64{3, 5},
			Data:    &proto.ColUInt32{65001, 65002, 65003, 65007, 65008},
		}

		if diff := helpers.Diff(*col, expected); diff != "" {
			t.Errorf("Initial DstASPath column state (-got, +want):\n%s", diff)
		}

		// Undo should remove the last array
		bf.Undo()

		expectedAfter := proto.ColArr[uint32]{
			Offsets: proto.ColUInt64{3},
			Data:    &proto.ColUInt32{65001, 65002, 65003},
		}

		if diff := helpers.Diff(*col, expectedAfter); diff != "" {
			t.Errorf("DstASPath column after undo (-got, +want):\n%s", diff)
		}
	})

}

func TestUndoArrayUInt128(t *testing.T) {
	c := NewMock(t)

	t.Run("one value", func(t *testing.T) {
		bf := c.NewFlowMessage()

		// Add Array(UInt128) value
		bf.AppendArrayUInt128(ColumnDstLargeCommunities, []UInt128{
			{High: (65401 << 32) + 100, Low: 200},
			{High: (65401 << 32) + 100, Low: 201},
		})

		// Check we have the expected initial state
		col := bf.batch.columns[ColumnDstLargeCommunities].(*proto.ColArr[proto.UInt128])
		expected := proto.ColArr[proto.UInt128]{
			Offsets: proto.ColUInt64{2},
			Data: &proto.ColUInt128{
				{High: (65401 << 32) + 100, Low: 200},
				{High: (65401 << 32) + 100, Low: 201},
			},
		}

		if diff := helpers.Diff(*col, expected); diff != "" {
			t.Errorf("Initial DstLargeCommunities column state (-got, +want):\n%s", diff)
		}

		// Undo should remove the array
		bf.Undo()

		expectedAfter := proto.ColArr[proto.UInt128]{
			Offsets: proto.ColUInt64{},
			Data:    &proto.ColUInt128{},
		}

		if diff := helpers.Diff(*col, expectedAfter); diff != "" {
			t.Errorf("DstLargeCommunities column after undo (-got, +want):\n%s", diff)
		}
	})

	t.Run("two values", func(t *testing.T) {
		bf := c.NewFlowMessage()

		// Add first Array(UInt128) value
		bf.AppendArrayUInt128(ColumnDstLargeCommunities, []UInt128{
			{High: (65401 << 32) + 100, Low: 200},
			{High: (65401 << 32) + 100, Low: 201},
		})
		bf.Finalize()

		// Add second Array(UInt128) value
		bf.AppendArrayUInt128(ColumnDstLargeCommunities, []UInt128{
			{High: (65402 << 32) + 100, Low: 300},
		})

		// Check we have the expected initial state
		col := bf.batch.columns[ColumnDstLargeCommunities].(*proto.ColArr[proto.UInt128])
		expected := proto.ColArr[proto.UInt128]{
			Offsets: proto.ColUInt64{2, 3},
			Data: &proto.ColUInt128{
				{High: (65401 << 32) + 100, Low: 200},
				{High: (65401 << 32) + 100, Low: 201},
				{High: (65402 << 32) + 100, Low: 300},
			},
		}

		if diff := helpers.Diff(*col, expected); diff != "" {
			t.Errorf("Initial DstLargeCommunities column state (-got, +want):\n%s", diff)
		}

		// Undo should remove the last array
		bf.Undo()

		expectedAfter := proto.ColArr[proto.UInt128]{
			Offsets: proto.ColUInt64{2},
			Data: &proto.ColUInt128{
				{High: (65401 << 32) + 100, Low: 200},
				{High: (65401 << 32) + 100, Low: 201},
			},
		}

		if diff := helpers.Diff(*col, expectedAfter); diff != "" {
			t.Errorf("DstLargeCommunities column after undo (-got, +want):\n%s", diff)
		}
	})
}

func TestBuildProtoInput(t *testing.T) {
	// Use a smaller version
	exporterAddress := netip.MustParseAddr("::ffff:203.0.113.14")
	c := NewMock(t)
	bf := c.NewFlowMessage()
	got := bf.ClickHouseProtoInput()

	bf.TimeReceived = 1000
	bf.SamplingRate = 20000
	bf.ExporterAddress = exporterAddress
	bf.AppendUint(ColumnDstAS, 65000)
	bf.AppendUint(ColumnBytes, 200)
	bf.AppendUint(ColumnPackets, 300)
	bf.Finalize()

	bf.Clear()

	bf.TimeReceived = 1002
	bf.ExporterAddress = exporterAddress
	bf.AppendUint(ColumnSrcAS, 65000)
	bf.AppendUint(ColumnBytes, 2000)
	bf.AppendUint(ColumnPackets, 30)
	bf.AppendUint(ColumnBytes, 300) // Duplicate!
	bf.Finalize()

	bf.TimeReceived = 1003
	bf.ExporterAddress = exporterAddress
	bf.AppendUint(ColumnSrcAS, 65001)
	bf.AppendUint(ColumnBytes, 202)
	bf.AppendUint(ColumnPackets, 3)
	bf.Finalize()

	// Let's compare a subset
	expected := proto.Input{
		{Name: "TimeReceived", Data: proto.ColDateTime{Data: []proto.DateTime{1002, 1003}}},
		{Name: "SrcAS", Data: proto.ColUInt32{65000, 65001}},
		{Name: "DstAS", Data: proto.ColUInt32{0, 0}},
		{Name: "Bytes", Data: proto.ColUInt64{2000, 202}},
		{Name: "Packets", Data: proto.ColUInt64{30, 3}},
	}
	got = slices.DeleteFunc(got, func(col proto.InputColumn) bool {
		return !slices.Contains([]string{"TimeReceived", "SrcAS", "DstAS", "Packets", "Bytes"}, col.Name)
	})
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("ClickHouseProtoInput() (-got, +want):\n%s", diff)
	}
}
