// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package filter parses and transforms a user filter.
package filter

import (
	"fmt"
	"net/netip"
	"strings"

	"akvorado/common/schema"
)

// Meta is used to inject/retrieve state from the parser.
type Meta struct {
	// ReverseDirection tells if we require the reverse direction for the provided filter (used as input)
	ReverseDirection bool
	// MainTableRequired tells if the main table is required to execute the expression (used as output)
	MainTableRequired bool
}

// ReverseColumnDirection reverts the direction of a provided column name.
func ReverseColumnDirection(name string) string {
	var candidate string
	if strings.HasPrefix(name, "Src") {
		candidate = "Dst" + name[3:]
	}
	if strings.HasPrefix(name, "Dst") {
		candidate = "Src" + name[3:]
	}
	if strings.HasPrefix(name, "In") {
		candidate = "Out" + name[2:]
	}
	if strings.HasPrefix(name, "Out") {
		candidate = "In" + name[3:]
	}
	if column, ok := schema.Flows.Columns.Get(candidate); ok {
		return column.Name
	}
	return name
}

// acceptColumn normalizes and returns the matched column name. It should be used
// in predicate code blocks.
func (c *current) acceptColumn() (string, error) {
	name := string(c.text)
	for _, columnName := range schema.Flows.Columns.Keys() {
		if strings.EqualFold(name, columnName) {
			if c.globalStore["meta"].(*Meta).ReverseDirection {
				return ReverseColumnDirection(columnName), nil
			}
			return columnName, nil
		}
	}
	return "", fmt.Errorf("unknown column %q", name)
}

// metaColumn remembers the matched column name in meta data. It should be used
// in state change blocks. Unfortunately, it cannot extract matched text, so it
// should be provided.
func (c *current) metaColumn(name string) error {
	if column, ok := schema.Flows.Columns.Get(name); ok {
		if column.MainOnly {
			c.state["main-table-only"] = true
		}
	}
	return nil
}

func lastIP(subnet netip.Prefix) netip.Addr {
	a16 := subnet.Addr().As16()
	var off uint8
	var bits uint8 = 128
	if subnet.Addr().Is4() {
		off = 12
		bits = 32
	}
	for b := uint8(subnet.Bits()); b < bits; b++ {
		byteNum, bitInByte := b/8, 7-(b%8)
		a16[off+byteNum] |= 1 << uint(bitInByte)
	}
	if subnet.Addr().Is4() {
		return netip.AddrFrom16(a16).Unmap()
	}
	return netip.AddrFrom16(a16)
}

func quote(v interface{}) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(toString(v)) + "'"
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}

func toString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	case uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", s)
	default:
		panic("not a string")
	}
}
