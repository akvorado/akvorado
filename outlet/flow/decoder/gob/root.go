// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

// Package gob handles gob decoding for testing purposes.
package gob

import (
	"bytes"
	"encoding/gob"
	"net/netip"
	"reflect"

	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/flow/decoder"
)

func init() {
	// Register types that may appear in OtherColumns for gob encoding/decoding
	gob.Register(schema.InterfaceBoundary(0))
}

// Decoder contains the state for the gob decoder.
type Decoder struct {
	r *reporter.Reporter
	d decoder.Dependencies
}

// New creates a new gob decoder.
func New(r *reporter.Reporter, dependencies decoder.Dependencies) decoder.Decoder {
	return &Decoder{
		r: r,
		d: dependencies,
	}
}

// Name returns the decoder name.
func (d *Decoder) Name() string {
	return "gob"
}

// Decode decodes a gob-encoded FlowMessage.
func (d *Decoder) Decode(in decoder.RawFlow, _ decoder.Option, bf *schema.FlowMessage, finalize decoder.FinalizeFlowFunc) (int, error) {
	var decoded schema.FlowMessage

	buf := bytes.NewReader(in.Payload)
	decoder := gob.NewDecoder(buf)

	if err := decoder.Decode(&decoded); err != nil {
		return 0, err
	}

	// We need to "replay" the decoded flow. We use reflection for this.
	decodedValue := reflect.ValueOf(&decoded).Elem()
	bfValue := reflect.ValueOf(bf).Elem()
	decodedType := decodedValue.Type()

	// Copy all public fields except OtherColumns
	for i := 0; i < decodedValue.NumField(); i++ {
		field := decodedType.Field(i)
		if !field.IsExported() || field.Name == "OtherColumns" {
			continue
		}

		sourceField := decodedValue.Field(i)
		targetField := bfValue.FieldByName(field.Name)
		if targetField.IsValid() && targetField.CanSet() {
			targetField.Set(sourceField)
		}
	}

	// Handle OtherColumns
	if decoded.OtherColumns != nil {
		for columnKey, value := range decoded.OtherColumns {
			switch v := value.(type) {
			case uint64:
				bf.AppendUint(columnKey, v)
			case uint32:
				bf.AppendUint(columnKey, uint64(v))
			case uint16:
				bf.AppendUint(columnKey, uint64(v))
			case uint8:
				bf.AppendUint(columnKey, uint64(v))
			case int:
				bf.AppendUint(columnKey, uint64(v))
			case string:
				bf.AppendString(columnKey, v)
			case netip.Addr:
				bf.AppendIPv6(columnKey, v)
			case schema.InterfaceBoundary:
				bf.AppendUint(columnKey, uint64(v))
			case []uint32:
				bf.AppendArrayUInt32(columnKey, v)
			case []schema.UInt128:
				bf.AppendArrayUInt128(columnKey, v)
			}
		}
	}

	finalize()
	return 1, nil
}
