// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package filter parses and transforms a user filter.
package filter

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
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

// condition is the right side of a comparison: an operator and a value. It is
// built before the left side is known, as the grammar matches the column first.
type condition struct {
	operator string
	value    sb.Expr
}

// apply completes the condition with its left side.
func (cond condition) apply(left sb.Expr) sb.Expr {
	return sb.Op(left, cond.operator, cond.value)
}

// meta returns the parser state.
func (c *current) meta() *Meta {
	return c.globalStore["meta"].(*Meta)
}

// reverseColumn returns the column for the opposite direction when the filter
// is parsed for the reverse direction. Otherwise, the column is returned as is.
func (c *current) reverseColumn(col schema.Column) schema.Column {
	meta := c.meta()
	if !meta.ReverseDirection {
		return col
	}
	var candidate string
	name := col.Name
	switch {
	case strings.HasPrefix(name, "Src"):
		candidate = "Dst" + name[3:]
	case strings.HasPrefix(name, "Dst"):
		candidate = "Src" + name[3:]
	case strings.HasPrefix(name, "In"):
		candidate = "Out" + name[2:]
	case strings.HasPrefix(name, "Out"):
		candidate = "In" + name[3:]
	}
	if column, ok := meta.Schema.LookupColumnByName(candidate); ok {
		return *column
	}
	return col
}

// column returns the expression for a column. The direction is reversed when
// needed and a column living only in the main table is recorded as such.
func (c *current) column(col schema.Column) sb.Expr {
	col = c.reverseColumn(col)
	if col.ClickHouseMainOnly {
		c.meta().MainTableRequired = true
	}
	return sb.Column(col.Name)
}

// value turns a value matched by the grammar into an expression. A column on
// the right side of a comparison is handled like any other column.
func (c *current) value(v any) sb.Expr {
	switch value := v.(type) {
	case schema.Column:
		return c.column(value)
	case uint8:
		return sb.Uint(uint64(value))
	case uint16:
		return sb.Uint(uint64(value))
	case uint32:
		return sb.Uint(uint64(value))
	case uint64:
		return sb.Uint(value)
	case sb.Expr:
		return value
	}
	// The grammar only produces the types above, including on its error paths.
	panic(fmt.Sprintf("unexpected value %v in filter", v))
}

// values turns a list matched by the grammar into expressions.
func (c *current) values(items []any) []sb.Expr {
	exprs := make([]sb.Expr, len(items))
	for i, item := range items {
		exprs[i] = c.value(item)
	}
	return exprs
}

// asExpr returns the expression matched by the grammar. On an error path, the
// parser hands back the partial match instead of an expression. The parse fails
// anyway, so no expression is good enough.
func asExpr(v any) sb.Expr {
	expr, _ := v.(sb.Expr)
	return expr
}

// combine joins the head expression with the tail matched by a repetition of
// "operator expression". The expression is the last item of each match.
func combine(join func(...sb.Expr) sb.Expr, head, rest any) sb.Expr {
	exprs := []sb.Expr{asExpr(head)}
	for _, item := range toSlice(rest) {
		parts := toSlice(item)
		exprs = append(exprs, asExpr(parts[len(parts)-1]))
	}
	return join(exprs...)
}

// acceptColumn normalizes and returns the matched column name. It should be
// used in action code blocks.
func (c *current) acceptColumn() (schema.Column, error) {
	name := string(c.text)
	sch := c.meta().Schema
	for _, column := range sch.Columns() {
		if strings.EqualFold(name, column.Name) {
			return column, nil
		}
	}
	return schema.Column{}, fmt.Errorf("unknown column %q", name)
}

// columnIsOfType returns true if the column is of one of the provided type. It
// should be used in predicate code blocks.
func (c *current) columnIsOfType(name any, types ...string) (bool, error) {
	columnName := name.(string)
	sch := c.meta().Schema
	for _, column := range sch.Columns() {
		if strings.EqualFold(columnName, column.Name) {
			return slices.Contains(types, column.ParserType), nil
		}
	}
	return false, nil
}

// getColumn gets a column by its name.
func (c *current) getColumn(name string) schema.Column {
	sch := c.meta().Schema
	if column, ok := sch.LookupColumnByName(name); ok {
		return *column
	}
	return schema.Column{}
}

// requireMainTable records that the main table is needed when one of the
// provided columns only lives there.
func (c *current) requireMainTable(cols ...schema.Column) {
	for _, col := range cols {
		if col.ClickHouseMainOnly {
			c.meta().MainTableRequired = true
		}
	}
}

// prefixCondition compares a column to a network prefix. When the prefix column
// is materialized, this is a direct comparison. Otherwise, the address range and
// the mask length are compared instead.
func (c *current) prefixCondition(col schema.Column, prefix netip.Prefix) sb.Expr {
	col = c.reverseColumn(col)
	if col.ClickHouseMaterialized {
		c.requireMainTable(col)
		return sb.Op(sb.Column(col.Name), "=", sb.String(prefix.String()))
	}
	direction := "Dst"
	if strings.HasPrefix(col.Name, "Src") {
		direction = "Src"
	}
	// Only the address and the mask are read, so only these two decide if the
	// main table is needed.
	addr := fmt.Sprintf("%sAddr", direction)
	mask := fmt.Sprintf("%sNetMask", direction)
	c.requireMainTable(c.getColumn(addr), c.getColumn(mask))
	return sb.And(
		betweenPrefix(sb.Column(addr), prefix, false),
		sb.Op(sb.Column(mask), "=", sb.Uint(uint64(prefix.Bits()))),
	)
}

// ipOrSubnetCondition builds the condition for a list mixing addresses and
// subnets. Addresses are gathered in a single IN, while each subnet needs its
// own range comparison.
func (c *current) ipOrSubnetCondition(col schema.Column, operator string, items []any) (sb.Expr, error) {
	left := c.column(col)
	addresses := []sb.Expr{}
	conditions := []sb.Expr{}
	for _, item := range items {
		switch value := item.(type) {
		case netip.Addr:
			addresses = append(addresses,
				sb.Function("toIPv6", sb.String(value.String())))
		case netip.Prefix:
			conditions = append(conditions, betweenPrefix(left, value, false))
		default:
			return sb.Expr{}, errors.New("expecting an IP address or a subnet")
		}
	}
	if len(addresses) > 0 {
		conditions = append([]sb.Expr{
			sb.Op(left, "IN", sb.Tuple(addresses...)),
		}, conditions...)
	}
	expr := sb.Or(conditions...)
	if operator == "NOT IN" {
		return sb.Not(expr), nil
	}
	if len(conditions) > 1 {
		return sb.Parens(expr), nil
	}
	return expr, nil
}

// protocolName turns a protocol number into its name, so a filter can compare
// it with a string. An unknown protocol becomes "???", like in the columns the
// console displays.
func protocolName(sch *schema.Component) func(sb.Expr) sb.Expr {
	return func(proto sb.Expr) sb.Expr {
		return sb.Function("dictGetOrDefault",
			sb.String(sch.DictionaryName(schema.DictionaryProtocols)), sb.String("name"),
			proto, sb.String("???"))
	}
}

// macToNum turns a MAC address into its numeric form.
func macToNum(mac any) sb.Expr {
	return sb.Function("MACStringToNum", sb.String(toString(mac)))
}

// macList turns a list of MAC addresses into their numeric forms.
func macList(items []any) []sb.Expr {
	exprs := make([]sb.Expr, len(items))
	for i, item := range items {
		exprs[i] = macToNum(item)
	}
	return exprs
}

// stringList turns a list of matched strings into string literals.
func stringList(items []any) []sb.Expr {
	exprs := make([]sb.Expr, len(items))
	for i, item := range items {
		exprs[i] = sb.String(toString(item))
	}
	return exprs
}

// largeCommunity packs the three parts of a large community into the single
// number stored in the database.
func largeCommunity(value1, value2, value3 uint32) sb.Expr {
	shift := func(value uint32, bits uint64) sb.Expr {
		return sb.Function("bitShiftLeft",
			sb.Cast(sb.Uint(uint64(value)), "UInt128"),
			sb.Uint(bits))
	}
	return sb.Op(
		sb.Op(shift(value1, 64), "+", shift(value2, 32)),
		"+",
		sb.Cast(sb.Uint(uint64(value3)), "UInt128"))
}

// largeCommunitiesColumn returns the large communities column matching a
// communities column.
func (c *current) largeCommunitiesColumn(col schema.Column) schema.Column {
	return c.getColumn(strings.Replace(col.Name, "Communities", "LargeCommunities", 1))
}

// betweenPrefix compares a column to the address range covered by a prefix.
func betweenPrefix(left sb.Expr, prefix netip.Prefix, not bool) sb.Expr {
	low := sb.Function("toIPv6", sb.String(prefix.Masked().Addr().String()))
	high := sb.Function("toIPv6", sb.String(lastIP(prefix).String()))
	if not {
		return sb.NotBetween(left, low, high)
	}
	return sb.Between(left, low, high)
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

func unquote(v any) string {
	charSlice := toSlice(v)
	var result strings.Builder
	for _, ch := range charSlice {
		s := toString(ch)
		if len(s) == 2 && s[0] == '\\' {
			// Escape sequence: \\ or \" or \', or anything else
			result.WriteByte(s[1])
		} else {
			result.WriteString(s)
		}
	}
	return result.String()
}

func toSlice(v any) []any {
	if v == nil {
		return nil
	}
	return v.([]any)
}

func toString(v any) string {
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
