// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import "github.com/netsampler/goflow2/v2/decoders/netflow"

type nfv9Header struct {
	Version        uint16
	Count          uint16
	SystemUptime   uint32
	UnixSeconds    uint32
	SequenceNumber uint32
	SourceID       uint32
}

type flowSetHeader netflow.FlowSetHeader

type templateRecordHeader struct {
	TemplateID uint16
	FieldCount uint16
}
type optionsTemplateRecordHeader struct {
	TemplateID   uint16
	ScopeLength  uint16
	OptionLength uint16
}

type templateField struct {
	// Pen is not handled
	Type   uint16
	Length uint16
}
