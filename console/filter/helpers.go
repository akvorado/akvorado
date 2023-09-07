// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package filter parses and transforms a user filter.
package filter

import (
	"errors"
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

// flattenExpr takes an expression and flattens it to a slice of strings. It
// also handles metadata for columns.
func (c *current) flattenExpr(expr []any, meta *Meta) []string {
	// Helpers for columns: reverse direction and extract metadata.
	reverseColumn := func(col schema.Column) schema.Column {
		name := col.Name
		if meta.ReverseDirection {
			var candidate string
			sch := c.globalStore["meta"].(*Meta).Schema
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
			if column, ok := sch.LookupColumnByName(candidate); ok {
				return *column
			}
		}
		return col
	}
	metaColumn := func(col schema.Column) schema.Column {
		col = reverseColumn(col)
		if col.ClickHouseMainOnly {
			meta.MainTableRequired = true
		}
		return col
	}
	var result []string // flattened, pre-join
	for i := range expr {
		switch s := expr[i].(type) {
		case schema.Column:
			s = metaColumn(s)
			result = append(result, s.Name)
		case string:
			result = append(result, s)
		case []any:
			result = append(result, c.flattenExpr(s, meta)...)
		case uint8, uint16, uint32, uint64, uint, int8, int16, int32, int64, int:
			result = append(result, fmt.Sprintf("%d", s))
		default:
			result = append(result, fmt.Sprintf("%s", s))
		}
	}
	return result
}

// compileExpr compile an expression to a SQL string. An expression is a nested
// slices of elements. Some elements are schema.Column and are reverted if
// needed and metadata-related information are extracted. Elements are flattened
// and joined with spaces (with a few exceptions).
func (c *current) compileExpr(expr []any, meta *Meta) string {
	var b strings.Builder
	sl := c.flattenExpr(expr, meta)
	for i := range sl {
		// Do we add a space between sl[i-1] and sl[i]?
		if i == 0 {
			b.WriteString(sl[0])
			continue
		}
		last := sl[i-1][len(sl[i-1])-1:]
		first := sl[i][:1]
		if first != "," && first != ")" && first != " " && last != "(" && last != " " {
			b.WriteString(" ")
		}
		b.WriteString(sl[i])
	}
	return b.String()
}

// acceptColumn normalizes and returns the matched column name. It should be
// used in predicate code blocks.
func (c *current) acceptColumn() (schema.Column, error) {
	name := string(c.text)
	sch := c.globalStore["meta"].(*Meta).Schema
	for _, column := range sch.Columns() {
		if strings.EqualFold(name, column.Name) {
			return column, nil
		}
	}
	return schema.Column{}, fmt.Errorf("unknown column %q", name)
}

func (c *current) acceptDynamicColumn() (schema.Column, error) {
	name := string(c.text)
	sch := c.globalStore["meta"].(*Meta).Schema
	for _, column := range sch.Columns() {
		if strings.EqualFold(name, column.Name) {
			// check, if we actually have a dynamic column. Dynamic column keys start at ColumnLast
			if column.Key < schema.ColumnLast {
				return schema.Column{}, fmt.Errorf("not a dynamic column: %q", name)
			}
			return column, nil
		}
	}
	return schema.Column{}, fmt.Errorf("unknown column %q", name)
}

// getColumn gets a column by its name.
func (c *current) getColumn(name string) schema.Column {
	sch := c.globalStore["meta"].(*Meta).Schema
	if column, ok := sch.LookupColumnByName(name); ok {
		return *column
	}
	return schema.Column{}
}

// parsePrefix parses a source or destination prefix to SQL.
func (c *current) parsePrefix(direction string) ([]any, error) {
	net, err := netip.ParsePrefix(string(c.text))
	if err != nil {
		return []any{}, errors.New("expecting a prefix")
	}
	prefix := "::ffff:"
	if net.Addr().Is6() {
		prefix = ""
	}
	return []any{
		fmt.Sprintf("BETWEEN toIPv6('%s%s') AND toIPv6('%s%s') AND",
			prefix, net.Masked().Addr().String(), prefix, lastIP(net).String()),
		c.getColumn(fmt.Sprintf("%sNetMask", direction)), "=", net.Bits(),
	}, nil
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
