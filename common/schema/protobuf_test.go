// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/ClickHouse/ch-go/proto"
	"google.golang.org/protobuf/encoding/protowire"
)

// populateBenchFlow fills a flow with a representative set of enriched fields.
// The dynamic columns go through Append* (and are dual-encoded to Protobuf when
// enabled); the fixed fields are appended by Finalize().
func populateBenchFlow(bf *FlowMessage) {
	bf.TimeReceived = 1000
	bf.SamplingRate = 30000
	bf.ExporterAddress = netip.MustParseAddr("2001:db8::1")
	bf.SrcAddr = netip.MustParseAddr("192.0.2.1")
	bf.DstAddr = netip.MustParseAddr("198.51.100.2")
	bf.SrcAS = 65001
	bf.DstAS = 65002
	bf.AppendUint(ColumnBytes, 1500)
	bf.AppendUint(ColumnPackets, 10)
	bf.AppendUint(ColumnSrcPort, 443)
	bf.AppendUint(ColumnDstPort, 33000)
	bf.AppendUint(ColumnProto, 6)
	bf.AppendUint(ColumnEType, 0x0800)
	bf.AppendString(ColumnExporterName, "router1.example.net")
}

// consumeProtobufFields parses a flat proto3 message into field number → values.
func consumeProtobufFields(t *testing.T, data []byte) map[protowire.Number][]any {
	t.Helper()
	out := map[protowire.Number][]any{}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			t.Fatalf("ConsumeTag: %v", protowire.ParseError(n))
		}
		data = data[n:]
		switch typ {
		case protowire.VarintType:
			v, m := protowire.ConsumeVarint(data)
			if m < 0 {
				t.Fatalf("ConsumeVarint: %v", protowire.ParseError(m))
			}
			out[num] = append(out[num], v)
			data = data[m:]
		case protowire.BytesType:
			v, m := protowire.ConsumeBytes(data)
			if m < 0 {
				t.Fatalf("ConsumeBytes: %v", protowire.ParseError(m))
			}
			out[num] = append(out[num], append([]byte(nil), v...))
			data = data[m:]
		default:
			t.Fatalf("unexpected wire type %d for field %d", typ, num)
		}
	}
	return out
}

func TestProtobufEncode(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()
	bf.EnableProtobuf()

	populateBenchFlow(bf)
	bf.Finalize()

	payload := bf.ProtobufMessage()
	if len(payload) == 0 {
		t.Fatal("ProtobufMessage() returned no bytes")
	}
	fields := consumeProtobufFields(t, payload)

	idxOf := func(key ColumnKey) protowire.Number {
		column, ok := c.LookupColumnByKey(key)
		if !ok || column.ProtobufIndex <= 0 {
			t.Fatalf("column %s has no Protobuf index", key)
		}
		return column.ProtobufIndex
	}
	wantVarint := func(key ColumnKey, want uint64) {
		got := fields[idxOf(key)]
		if len(got) != 1 || got[0].(uint64) != want {
			t.Errorf("%s: got %v, want varint %d", key, got, want)
		}
	}
	wantBytes := func(key ColumnKey, want []byte) {
		got := fields[idxOf(key)]
		if len(got) != 1 {
			t.Fatalf("%s: got %v, want one value", key, got)
		}
		if string(got[0].([]byte)) != string(want) {
			t.Errorf("%s: got %x, want %x", key, got[0], want)
		}
	}

	// Dynamic columns appended during "enrichment".
	wantVarint(ColumnBytes, 1500)
	wantVarint(ColumnPackets, 10)
	wantVarint(ColumnSrcPort, 443)
	wantVarint(ColumnDstPort, 33000)
	wantVarint(ColumnProto, 6)
	wantVarint(ColumnEType, 0x0800)
	// Fixed fields appended by Finalize().
	wantVarint(ColumnTimeReceived, 1000)
	wantVarint(ColumnSamplingRate, 30000)
	wantVarint(ColumnSrcAS, 65001)
	wantVarint(ColumnDstAS, 65002)
	exporter := netip.MustParseAddr("2001:db8::1").As16()
	wantBytes(ColumnExporterAddress, exporter[:])
	src := netip.MustParseAddr("192.0.2.1").As16()
	wantBytes(ColumnSrcAddr, src[:])
}

// TestProtobufEncodeArrays covers the repeated-field encoders: Array(UInt32)
// columns (AS path, communities) emit one varint per element and Array(UInt128)
// columns (large communities) emit one 16-byte value per element (high then low,
// big-endian).
func TestProtobufEncodeArrays(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()
	bf.EnableProtobuf()

	asPath := []uint32{65001, 65002, 65003}
	communities := []uint32{0x00010002, 0x00030004}
	largeComm := []UInt128{
		{High: 65001, Low: 0x0000000100000002},
		{High: 65002, Low: 0x0000000300000004},
	}
	bf.AppendArrayUInt32(ColumnDstASPath, asPath)
	bf.AppendArrayUInt32(ColumnDstCommunities, communities)
	bf.AppendArrayUInt128(ColumnDstLargeCommunities, largeComm)
	bf.Finalize()

	fields := consumeProtobufFields(t, bf.ProtobufMessage())
	idxOf := func(key ColumnKey) protowire.Number {
		column, ok := c.LookupColumnByKey(key)
		if !ok || column.ProtobufIndex <= 0 {
			t.Fatalf("column %s has no Protobuf index", key)
		}
		return column.ProtobufIndex
	}
	wantVarints := func(key ColumnKey, want []uint32) {
		got := fields[idxOf(key)]
		if len(got) != len(want) {
			t.Fatalf("%s: got %d values, want %d", key, len(got), len(want))
		}
		for i, w := range want {
			if got[i].(uint64) != uint64(w) {
				t.Errorf("%s[%d]: got %v, want %d", key, i, got[i], w)
			}
		}
	}
	wantVarints(ColumnDstASPath, asPath)
	wantVarints(ColumnDstCommunities, communities)

	got := fields[idxOf(ColumnDstLargeCommunities)]
	if len(got) != len(largeComm) {
		t.Fatalf("DstLargeCommunities: got %d values, want %d", len(got), len(largeComm))
	}
	for i, w := range largeComm {
		var want [16]byte
		binary.BigEndian.PutUint64(want[0:8], w.High)
		binary.BigEndian.PutUint64(want[8:16], w.Low)
		if b := got[i].([]byte); string(b) != string(want[:]) {
			t.Errorf("DstLargeCommunities[%d]: got %x, want %x", i, b, want)
		}
	}
}

// TestProtobufMessageHash checks the hash is stable for a schema and is the same
// value embedded in the generated message name, so consumers can key on it.
func TestProtobufMessageHash(t *testing.T) {
	c := NewMock(t)
	hash := c.ProtobufMessageHash()
	if hash == "" {
		t.Fatal("ProtobufMessageHash() returned empty string")
	}
	if again := c.ProtobufMessageHash(); again != hash {
		t.Errorf("ProtobufMessageHash() not stable: %q then %q", hash, again)
	}
	if want := "FlowMessagev" + hash; !strings.Contains(c.ProtobufDefinition(), want) {
		t.Errorf("definition missing message name %q", want)
	}
}

// TestProtobufDefinitionMatchesEncoding guards the invariant that the published
// .proto definition and the actual encoder agree on which columns are exported.
// The encoder emits a field iff ProtobufIndex > 0; the .proto generator emits a
// field iff ProtobufIndex > 0 AND its wire type renders to a name. If those ever
// diverge, a column would be encoded on the wire but absent from the definition
// consumers rely on (or vice versa).
func TestProtobufDefinitionMatchesEncoding(t *testing.T) {
	c := NewMock(t).EnableAllColumns()
	def := c.ProtobufDefinition()
	for _, column := range c.Columns() {
		if column.ProtobufIndex <= 0 {
			continue
		}
		// Encoded columns must have a type the .proto generator can render.
		if protobufTypeName(column.ProtobufType) == "" {
			t.Errorf("column %s (index %d) is encoded but has no .proto type (kind %v)",
				column.Name, column.ProtobufIndex, column.ProtobufType)
			continue
		}
		// ...and must actually appear in the generated definition.
		decl := fmt.Sprintf(" %s = %d;", column.Name, column.ProtobufIndex)
		if !strings.Contains(def, decl) {
			t.Errorf("column %s missing from .proto definition (expected %q)", column.Name, decl)
		}
	}
}

// TestProtobufDisabledIsZeroCost confirms that with Protobuf off, no message is
// produced and the disabled flag short-circuits the encoder.
func TestProtobufDisabledIsZeroCost(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()
	populateBenchFlow(bf)
	bf.Finalize()
	if payload := bf.ProtobufMessage(); payload != nil {
		t.Errorf("expected no Protobuf message when disabled, got %d bytes", len(payload))
	}
}

// marshalFlowJSONReadback replicates solution 1 (the draft branch): it rebuilds
// a flat JSON object by reading the last appended value of each ClickHouse
// column. Kept here, test-only, purely to benchmark it head-to-head against the
// append-time Protobuf encoder.
func (bf *FlowMessage) marshalFlowJSONReadback() ([]byte, error) {
	m := make(map[string]any, 24)
	if bf.TimeReceived != 0 {
		m["TimeReceived"] = bf.TimeReceived
	}
	if bf.SamplingRate != 0 {
		m["SamplingRate"] = bf.SamplingRate
	}
	if bf.ExporterAddress.IsValid() {
		m["ExporterAddress"] = bf.ExporterAddress.Unmap().String()
	}
	if bf.SrcAddr.IsValid() {
		m["SrcAddr"] = bf.SrcAddr.Unmap().String()
	}
	if bf.DstAddr.IsValid() {
		m["DstAddr"] = bf.DstAddr.Unmap().String()
	}
	if bf.SrcAS != 0 {
		m["SrcAS"] = bf.SrcAS
	}
	if bf.DstAS != 0 {
		m["DstAS"] = bf.DstAS
	}
	for _, column := range bf.schema.Columns() {
		if column.Disabled || !bf.batch.columnSet.Test(uint(column.Key)) {
			continue
		}
		col := bf.batch.columns[column.Key]
		if col == nil {
			continue
		}
		if v, ok := lastColumnValueForBench(col); ok {
			m[column.Name] = v
		}
	}
	return json.Marshal(m)
}

func lastColumnValueForBench(c proto.Column) (any, bool) {
	switch col := c.(type) {
	case *proto.ColUInt64:
		if n := len(*col); n > 0 {
			return (*col)[n-1], true
		}
	case *proto.ColUInt32:
		if n := len(*col); n > 0 {
			return (*col)[n-1], true
		}
	case *proto.ColUInt16:
		if n := len(*col); n > 0 {
			return (*col)[n-1], true
		}
	case *proto.ColUInt8:
		if n := len(*col); n > 0 {
			return (*col)[n-1], true
		}
	case *proto.ColEnum8:
		if n := len(*col); n > 0 {
			return uint8((*col)[n-1]), true
		}
	case *proto.ColLowCardinality[string]:
		if n := len(col.Values); n > 0 {
			return col.Values[n-1], true
		}
	}
	return nil, false
}

var benchSink int

func BenchmarkEncodeBaseline(b *testing.B) {
	c := NewMock(b)
	bf := c.NewFlowMessage()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		populateBenchFlow(bf)
		bf.Finalize()
	}
	_ = bf
}

func BenchmarkEncodeProtobuf(b *testing.B) {
	c := NewMock(b)
	bf := c.NewFlowMessage()
	bf.EnableProtobuf()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		populateBenchFlow(bf)
		bf.Finalize()
		benchSink += len(bf.ProtobufMessage())
	}
}

func BenchmarkEncodeJSONReadback(b *testing.B) {
	c := NewMock(b)
	bf := c.NewFlowMessage()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		populateBenchFlow(bf)
		payload, err := bf.marshalFlowJSONReadback()
		if err != nil {
			b.Fatal(err)
		}
		benchSink += len(payload)
		bf.Finalize()
	}
}

func TestProtobufVsJSONSize(t *testing.T) {
	c := NewMock(t)
	bf := c.NewFlowMessage()
	bf.EnableProtobuf()
	populateBenchFlow(bf)
	bf.Finalize()
	pbSize := len(bf.ProtobufMessage())

	bf2 := c.NewFlowMessage()
	populateBenchFlow(bf2)
	jsonBytes, _ := bf2.marshalFlowJSONReadback()
	t.Logf("wire payload: protobuf=%d bytes, json=%d bytes (%.1fx)", pbSize, len(jsonBytes), float64(len(jsonBytes))/float64(pbSize))
}
