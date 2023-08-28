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
	// Schema is the data schema (used as input)
	Schema *schema.Component
	// ReverseDirection tells if we require the reverse direction for the provided filter (used as input)
	ReverseDirection bool
	// MainTableRequired tells if the main table is required to execute the expression (used as output)
	MainTableRequired bool
}

const mainTableOnlyMarker = "@@M!M...🫠@@"

// extractMeta will extract metadata from parsed expression.
func (c *current) extractMeta(expr string, meta *Meta) string {
	newExpr := strings.Replace(expr, mainTableOnlyMarker, "", -1)
	if len(newExpr) != len(expr) {
		meta.MainTableRequired = true
	}
	return newExpr
}

// encodeMeta encode metadata inside column name.
func encodeMeta(column schema.Column) string {
	if !column.ClickHouseMainOnly {
		return column.Name
	}
	return fmt.Sprintf("%s%s", column.Name, mainTableOnlyMarker)
}

// acceptColumn normalizes and returns the matched column name. It should be
// used in predicate code blocks. It uses a special marker if the column
// requires the main table. This is quite hacky and it should be ensured that
// the column name is preserved during parsing (notably for rules we know the
// column name, we should still use the output of this function).
func (c *current) acceptColumn() (string, error) {
	name := string(c.text)
	schema := c.globalStore["meta"].(*Meta).Schema
	for _, column := range schema.Columns() {
		if strings.EqualFold(name, column.Name) {
			if c.globalStore["meta"].(*Meta).ReverseDirection {
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
				if column, ok := schema.LookupColumnByName(candidate); ok {
					return encodeMeta(*column), nil
				}
			}
			return encodeMeta(column), nil
		}
	}
	return "", fmt.Errorf("unknown column %q", name)
}

// metaColumn remembers the matched column name in meta data. It should be used
// in state change blocks. Unfortunately, it cannot extract matched text, so it
// should be provided.
func (c *current) metaColumn(name string) error {
	if column, ok := c.globalStore["meta"].(*Meta).Schema.LookupColumnByName(name); ok {
		if column.ClickHouseMainOnly {
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
