// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package query provides query columns and query filters. These
// types are special as they need a schema to be validated.
package query

import (
	"fmt"
	"strings"

	"akvorado/common/constants"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
)

// Column represents a query column. It should be instantiated with NewColumn() or
// Unmarshal(), then call Validate().
type Column struct {
	validated bool
	name      string
	key       schema.ColumnKey
}

// Columns is a set of query columns.
type Columns []Column

// NewColumn creates a new column. Validate() should be called before using it.
func NewColumn(name string) Column {
	return Column{name: name}
}

func (qc Column) check() {
	if !qc.validated {
		panic("query column not validated")
	}
}

func (qc Column) String() string {
	return qc.name
}

// Equal returns true iff two columns have the same name.
func (qc Column) Equal(oqc Column) bool {
	return qc.name == oqc.name
}

// MarshalText turns a column into a string.
func (qc Column) MarshalText() ([]byte, error) {
	return []byte(qc.name), nil
}

// UnmarshalText parses a column. Validate() should be called before use.
func (qc *Column) UnmarshalText(input []byte) error {
	name := string(input)
	*qc = Column{name: name}
	return nil
}

// Key returns the key for the column.
func (qc *Column) Key() schema.ColumnKey {
	qc.check()
	return qc.key
}

// Validate should be called before using the column. We need a schema component
// for that.
func (qc *Column) Validate(schema *schema.Component) error {
	if column, ok := schema.LookupColumnByName(qc.name); ok && !column.ConsoleNotDimension && !column.Disabled {
		qc.key = column.Key
		qc.validated = true
		return nil
	}
	return fmt.Errorf("unknown column name %s", qc.name)
}

// Reverse reverses the column direction
func (qc *Column) Reverse(schema *schema.Component) {
	name := schema.ReverseColumnDirection(qc.Key()).String()
	reverted := Column{name: name}
	if reverted.Validate(schema) == nil {
		*qc = reverted
	}
	// No modification otherwise
}

// Reverse reverses the direction of all columns
func (qcs Columns) Reverse(schema *schema.Component) {
	for i := range qcs {
		qcs[i].Reverse(schema)
	}
}

// Validate call Validate on each column.
func (qcs Columns) Validate(schema *schema.Component) error {
	for i := range qcs {
		if err := qcs[i].Validate(schema); err != nil {
			return err
		}
	}
	return nil
}

// ToSQLSelect transforms a column into an expression to use in SELECT
func (qc Column) ToSQLSelect(sch *schema.Component) sb.Expr {
	key := qc.Key()
	self := sb.Column(qc.String())
	switch key {
	// Special cases
	case schema.ColumnSrcAS, schema.ColumnDstAS, schema.ColumnDst1stAS, schema.ColumnDst2ndAS, schema.ColumnDst3rdAS:
		return sb.Function("concat",
			sb.Function("toString", self),
			sb.String(": "),
			self.Apply(DictionaryName(schema.DictionaryASNs, "???")))
	case schema.ColumnInIfBoundary, schema.ColumnOutIfBoundary:
		return sb.Function("toString", self)
	case schema.ColumnEType:
		etype := sb.Column(schema.ColumnEType.String())
		return sb.Function("if",
			sb.Op(etype, "=", sb.Uint(constants.ETypeIPv4)),
			sb.String("IPv4"),
			sb.Function("if",
				sb.Op(etype, "=", sb.Uint(constants.ETypeIPv6)),
				sb.String("IPv6"),
				sb.String("???")))
	case schema.ColumnProto:
		return self.Apply(DictionaryName(schema.DictionaryProtocols, "???"))
	case schema.ColumnMPLSLabels, schema.ColumnDstASPath:
		return sb.Function("arrayStringConcat", self, sb.String(" "))
	case schema.ColumnSrcCommunities, schema.ColumnDstCommunities:
		direction := qc.String()[:3]
		return sb.Function("arrayStringConcat",
			sb.Function("arrayConcat",
				CommunitiesToStrings(fmt.Sprintf("%sCommunities", direction)),
				LargeCommunitiesToStrings(fmt.Sprintf("%sLargeCommunities", direction))),
			sb.String(" "))
	case schema.ColumnSrcMAC, schema.ColumnDstMAC:
		return sb.Function("MACNumToString", self)
	case schema.ColumnTCPFlags:
		bits := []string{
			"FIN",
			"SYN",
			"RST",
			"PSH",
			".ACK",
			"URG",
			"ECE",
			"CWR",
			"NS",
		}
		flags := make([]sb.Expr, len(bits))
		for bit, name := range bits {
			flags[bit] = sb.Function("if",
				sb.Op(
					sb.Function("bitTest", self, sb.Uint(uint64(bit))),
					"=", sb.Uint(1)),
				sb.String(name[:1]),
				sb.String(""))
		}
		return sb.Function("arrayStringConcat",
			sb.Array(flags...), sb.String(""))
	case schema.ColumnDstPort, schema.ColumnSrcPort:
		proto := sb.Column(schema.ColumnProto.String())
		named := func(dictionary string) sb.Expr {
			return sb.Function("concat",
				sb.Function("toString", self),
				sb.String("/"),
				self.Apply(DictionaryName(dictionary, "")))
		}
		// The port may have no name in the dictionary, in which case the
		// trailing slash is dropped.
		return sb.Function("replaceRegexpOne",
			sb.Function("multiIf",
				sb.Op(proto, "=", sb.Uint(constants.ProtoTCP)),
				named(schema.DictionaryTCP),
				sb.Op(proto, "=", sb.Uint(constants.ProtoUDP)),
				named(schema.DictionaryUDP),
				sb.Function("toString", self)),
			sb.String("/$"), sb.String(""))

	// Generic cases
	default:
		if col, ok := sch.LookupColumnByKey(key); ok {
			if strings.HasPrefix(col.ClickHouseType, "UInt") {
				return sb.Function("toString", self)
			}
			if col.ClickHouseType == "IPv6" || col.ClickHouseType == "LowCardinality(IPv6)" {
				return self.Apply(AddressToString)
			}
		}
		return self
	}
}

// DictionaryName looks up the name matching a key in a ClickHouse dictionary.
// When the key is unknown, the fallback is used instead.
func DictionaryName(dictionary, fallback string) func(sb.Expr) sb.Expr {
	return func(key sb.Expr) sb.Expr {
		return sb.Function("dictGetOrDefault",
			sb.String(dictionary), sb.String("name"),
			key, sb.String(fallback))
	}
}

// AddressToString turns an address into a string. All addresses are stored as
// IPv6 ones, so the prefix of an IPv4-mapped address is removed.
func AddressToString(addr sb.Expr) sb.Expr {
	return sb.Function("replaceRegexpOne",
		sb.Function("IPv6NumToString", addr),
		sb.String("^::ffff:"), sb.String(""))
}

// CommunitiesToStrings turns an array of BGP communities into an array of
// strings using the "AS:value" notation.
func CommunitiesToStrings(column string) sb.Expr {
	return sb.Function("arrayMap",
		sb.Lambda("c", sb.Function("concat",
			sb.Function("toString",
				sb.Function("bitShiftRight", sb.Column("c"), sb.Uint(16))),
			sb.String(":"),
			sb.Function("toString",
				sb.Function("bitAnd", sb.Column("c"), sb.Number("0xffff"))))),
		sb.Column(column))
}

// LargeCommunitiesToStrings turns an array of large BGP communities into an
// array of strings using the "AS:value:value" notation.
func LargeCommunitiesToStrings(column string) sb.Expr {
	part := func(shift uint64) sb.Expr {
		value := sb.Column("c")
		if shift > 0 {
			value = sb.Function("bitShiftRight", value, sb.Uint(shift))
		}
		return sb.Function("toString",
			sb.Function("bitAnd", value, sb.Number("0xffffffff")))
	}
	return sb.Function("arrayMap",
		sb.Lambda("c", sb.Function("concat",
			part(64), sb.String(":"),
			part(32), sb.String(":"),
			part(0))),
		sb.Column(column))
}
