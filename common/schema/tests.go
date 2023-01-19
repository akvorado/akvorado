// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package schema

import (
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"akvorado/common/helpers"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/protobuf/encoding/protowire"
)

var debug = true

// DisableDebug disables debug during the provided test. We keep this as a
// global function for performance reason (when release, debug is a const).
func DisableDebug(t testing.TB) {
	debug = false
	t.Cleanup(func() {
		debug = true
	})
}

// NewMock create a new schema component.
func NewMock(t testing.TB) *Component {
	t.Helper()
	c, err := New()
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}

// ProtobufDecode decodes the provided protobuf message.
func (schema *Schema) ProtobufDecode(t *testing.T, input []byte) *FlowMessage {
	parser := protoparse.Parser{
		Accessor: protoparse.FileContentsFromMap(map[string]string{
			"flow.proto": schema.ProtobufDefinition(),
		}),
	}
	descs, err := parser.ParseFiles("flow.proto")
	if err != nil {
		t.Fatalf("ParseFiles(%q) error:\n%+v", "flow.proto", err)
	}

	var descriptor *desc.MessageDescriptor
	for _, msg := range descs[0].GetMessageTypes() {
		if strings.HasPrefix(msg.GetName(), "FlowMessagev") {
			descriptor = msg
			break
		}
	}
	if descriptor == nil {
		t.Fatal("cannot find message descriptor")
	}

	message := dynamic.NewMessage(descriptor)
	size, n := protowire.ConsumeVarint(input)
	if len(input)-n != int(size) {
		t.Fatalf("bad length for protobuf message: %d - %d != %d", len(input), n, size)
	}
	if err := message.Unmarshal(input[n:]); err != nil {
		t.Fatalf("Unmarshal() error:\n%+v", err)
	}
	textVersion, _ := message.MarshalTextIndent()
	t.Logf("Unmarshal():\n%s", textVersion)

	flow := FlowMessage{
		ProtobufDebug: map[ColumnKey]interface{}{},
	}
	for _, field := range message.GetKnownFields() {
		k := int(field.GetNumber())
		name := field.GetName()
		switch name {
		case "TimeReceived":
			flow.TimeReceived = message.GetFieldByNumber(k).(uint64)
		case "SamplingRate":
			flow.SamplingRate = uint32(message.GetFieldByNumber(k).(uint64))
		case "ExporterAddress":
			ip, _ := netip.AddrFromSlice(message.GetFieldByNumber(k).([]byte))
			flow.ExporterAddress = ip
		case "SrcAddr":
			ip, _ := netip.AddrFromSlice(message.GetFieldByNumber(k).([]byte))
			flow.SrcAddr = ip
		case "DstAddr":
			ip, _ := netip.AddrFromSlice(message.GetFieldByNumber(k).([]byte))
			flow.DstAddr = ip
		case "SrcAS":
			flow.SrcAS = uint32(message.GetFieldByNumber(k).(uint32))
		case "DstAS":
			flow.DstAS = uint32(message.GetFieldByNumber(k).(uint32))
		default:
			column, ok := schema.LookupColumnByName(name)
			if !ok {
				break
			}
			key := column.Key
			value := message.GetFieldByNumber(k)
			if reflect.ValueOf(value).IsZero() {
				break
			}
			flow.ProtobufDebug[key] = value
		}
	}

	return &flow
}

// EnableAllColumns enable all columns and returns itself.
func (schema *Component) EnableAllColumns() *Component {
	for i := range schema.columns {
		schema.columns[i].Disabled = false
	}
	return schema
}

func init() {
	helpers.AddPrettyFormatter(reflect.TypeOf(ColumnBytes), fmt.Sprint)
}
