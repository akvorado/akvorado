// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"akvorado/common/schema"

	"github.com/netsampler/goflow2/v3/decoders/netflow"
)

// Application attributes, in the order of applicationColumns.
const (
	applicationName = iota
	applicationCategory
	applicationSubCategory
	applicationGroup
	applicationTrafficClass
	applicationBusinessRelevance
	applicationFamily
	applicationSet
	applicationP2P
	applicationTunnel
	applicationEncrypted
	applicationAttributeCount
)

// applicationColumns maps application attributes to their column.
var applicationColumns = [applicationAttributeCount]schema.ColumnKey{
	schema.ColumnApplication,
	schema.ColumnApplicationCategory,
	schema.ColumnApplicationSubCategory,
	schema.ColumnApplicationGroup,
	schema.ColumnApplicationTrafficClass,
	schema.ColumnApplicationBusinessRelevance,
	schema.ColumnApplicationFamily,
	schema.ColumnApplicationSet,
	schema.ColumnApplicationP2P,
	schema.ColumnApplicationTunnel,
	schema.ColumnApplicationEncrypted,
}

const ciscoPEN = 9

// IANA information elements for application attributes.
var ianaApplicationAttributes = map[uint16]int{
	netflow.IPFIX_FIELD_applicationName:            applicationName,
	netflow.IPFIX_FIELD_p2pTechnology:              applicationP2P,
	netflow.IPFIX_FIELD_tunnelTechnology:           applicationTunnel,
	netflow.IPFIX_FIELD_encryptedTechnology:        applicationEncrypted,
	netflow.IPFIX_FIELD_applicationCategoryName:    applicationCategory,
	netflow.IPFIX_FIELD_applicationSubCategoryName: applicationSubCategory,
	netflow.IPFIX_FIELD_applicationGroupName:       applicationGroup,
}

// ciscoApplicationAttributes maps Cisco enterprise-specific information
// elements (PEN 9) for application attributes. With NetFlow v9, Cisco exports
// them with a field type of 32768 plus the element ID.
var ciscoApplicationAttributes = map[uint16]int{
	12230: applicationFamily,
	12231: applicationSet,
	12232: applicationCategory,
	12233: applicationSubCategory,
	12234: applicationGroup,
	12243: applicationTrafficClass,
	12244: applicationBusinessRelevance,
}

// applicationAttribute returns the application attribute of an option field.
func applicationAttribute(version uint16, field netflow.DataField) (int, bool) {
	var attribute int
	var ok bool
	switch {
	case field.PenProvided && field.Pen == ciscoPEN:
		// Some decoders keep the enterprise bit in the type.
		attribute, ok = ciscoApplicationAttributes[field.Type&^0x8000]
	case field.PenProvided:
	case version == 9 && field.Type >= 0x8000:
		attribute, ok = ciscoApplicationAttributes[field.Type-0x8000]
	default:
		attribute, ok = ianaApplicationAttributes[field.Type]
	}
	return attribute, ok
}

// decodeApplicationOptions records the application attributes from an option
// record, if it contains an application ID.
func decodeApplicationOptions(
	version uint16,
	tao *templatesAndOptions,
	fields []netflow.DataField,
) {
	var (
		id     []byte
		update application
		found  bool
	)
	for _, field := range fields {
		v, ok := field.Value.([]byte)
		if !ok {
			continue
		}
		if !field.PenProvided && field.Type == netflow.IPFIX_FIELD_applicationId {
			id = v
			continue
		}
		if attribute, ok := applicationAttribute(version, field); ok {
			update[attribute] = decodeString(v)
			found = true
		}
	}
	if len(id) > 0 && found {
		tao.UpdateApplication(version, id, &update)
	}
}

// appendApplication appends the application columns for an application ID.
func appendApplication(
	bf *schema.FlowMessage,
	tao *templatesAndOptions,
	version uint16,
	id []byte,
) {
	app, _ := tao.GetApplication(version, id)
	if app[applicationName] == "" {
		app[applicationName] = formatApplicationID(id)
	}
	for attribute, value := range app {
		if value != "" {
			bf.AppendString(applicationColumns[attribute], value)
		}
	}
}

// formatApplicationID formats an application ID as defined in RFC 6759,
// section 4: a classification engine ID, then a selector ID. For engine 20
// (PANA-L7-PEN), the selector is preceded by an enterprise number.
func formatApplicationID(id []byte) string {
	if len(id) < 2 {
		return hex.EncodeToString(id)
	}
	engine, selector := id[0], id[1:]
	if engine == 20 && len(selector) > 4 {
		return fmt.Sprintf("%d:%d:%d", engine,
			binary.BigEndian.Uint32(selector[:4]), decodeUNumber(selector[4:]))
	}
	if len(selector) > 8 {
		return fmt.Sprintf("%d:%x", engine, selector)
	}
	return fmt.Sprintf("%d:%d", engine, decodeUNumber(selector))
}

// decodeString decodes a string information element. Fixed-size strings are
// padded with NUL bytes.
func decodeString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(bytes.TrimSpace(b))
}
