// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package filter parses and transforms a user filter.
package filter

import (
	"fmt"
	"net/netip"
	"strings"
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
	if strings.HasPrefix(name, "Src") {
		return "Dst" + name[3:]
	}
	if strings.HasPrefix(name, "Dst") {
		return "Src" + name[3:]
	}
	if strings.HasPrefix(name, "In") {
		return "Out" + name[2:]
	}
	if strings.HasPrefix(name, "Out") {
		return "In" + name[3:]
	}
	return name
}

func (c *current) reverseColumnDirection(name string) string {
	if c.globalStore["meta"].(*Meta).ReverseDirection {
		return ReverseColumnDirection(name)
	}
	return name
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
