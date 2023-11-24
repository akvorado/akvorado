// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package query provides query columns and query filters. These
// types are special as they need a schema to be validated.
package query

import (
	"fmt"
	"strings"

	"akvorado/common/helpers"
	"akvorado/common/schema"
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
func (qc Column) ToSQLSelect(sch *schema.Component) string {
	var strValue string
	key := qc.Key()
	switch key {
	// Special cases
	case schema.ColumnSrcAS, schema.ColumnDstAS, schema.ColumnDst1stAS, schema.ColumnDst2ndAS, schema.ColumnDst3rdAS:
		strValue = fmt.Sprintf(`concat(toString(%s), ': ', dictGetOrDefault('asns', 'name', %s, '???'))`,
			qc, qc)
	case schema.ColumnEType:
		strValue = fmt.Sprintf(`if(EType = %d, 'IPv4', if(EType = %d, 'IPv6', '???'))`,
			helpers.ETypeIPv4, helpers.ETypeIPv6)
	case schema.ColumnProto:
		strValue = `dictGetOrDefault('protocols', 'name', Proto, '???')`
	case schema.ColumnMPLSLabels:
		strValue = `arrayStringConcat(MPLSLabels, ' ')`
	case schema.ColumnDstASPath:
		strValue = `arrayStringConcat(DstASPath, ' ')`
	case schema.ColumnDstCommunities:
		strValue = `arrayStringConcat(arrayConcat(arrayMap(c -> concat(toString(bitShiftRight(c, 16)), ':', toString(bitAnd(c, 0xffff))), DstCommunities), arrayMap(c -> concat(toString(bitAnd(bitShiftRight(c, 64), 0xffffffff)), ':', toString(bitAnd(bitShiftRight(c, 32), 0xffffffff)), ':', toString(bitAnd(c, 0xffffffff))), DstLargeCommunities)), ' ')`
	case schema.ColumnSrcMAC, schema.ColumnDstMAC:
		strValue = fmt.Sprintf("MACNumToString(%s)", qc)
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
		array := make([]string, len(bits))
		for bit, v := range bits {
			array[bit] = fmt.Sprintf("if(bitTest(%s, %d) = 1, '%s', '')", qc, bit, v[:1])
		}
		strValue = fmt.Sprintf("arrayStringConcat([%s], '')", strings.Join(array, ", "))

	// Generic cases
	default:
		strValue = qc.String()
		if col, ok := sch.LookupColumnByKey(key); ok {
			if strings.HasPrefix(col.ClickHouseType, "UInt") {
				strValue = fmt.Sprintf(`toString(%s)`, qc)
			} else if col.ClickHouseType == "IPv6" || col.ClickHouseType == "LowCardinality(IPv6)" {
				strValue = fmt.Sprintf("replaceRegexpOne(IPv6NumToString(%s), '^::ffff:', '')", qc)
			}
		}
	}
	return strValue
}
