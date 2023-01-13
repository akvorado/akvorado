// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"errors"
	"fmt"
	"strings"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/filter"
)

type queryColumn schema.ColumnKey

func (qc queryColumn) MarshalText() ([]byte, error) {
	return []byte(schema.ColumnKey(qc).String()), nil
}
func (qc queryColumn) String() string {
	return schema.ColumnKey(qc).String()
}
func (qc *queryColumn) UnmarshalText(input []byte) error {
	name := string(input)
	if column, ok := schema.Flows.LookupColumnByName(name); ok && !column.ConsoleNotDimension {
		*qc = queryColumn(column.Key)
		return nil
	}
	return errors.New("unknown field")
}

func requireMainTable(qcs []queryColumn, qf queryFilter) bool {
	if qf.MainTableRequired {
		return true
	}
	for _, qc := range qcs {
		if column, ok := schema.Flows.Columns.Get(schema.ColumnKey(qc)); ok && column.MainOnly {
			return true
		}
	}
	return false
}

type queryFilter struct {
	Filter            string
	ReverseFilter     string
	MainTableRequired bool
}

func (qf queryFilter) String() string {
	return qf.Filter
}
func (qf queryFilter) MarshalText() ([]byte, error) {
	return []byte(qf.Filter), nil
}
func (qf *queryFilter) UnmarshalText(input []byte) error {
	if strings.TrimSpace(string(input)) == "" {
		*qf = queryFilter{}
		return nil
	}
	meta := &filter.Meta{}
	direct, err := filter.Parse("", input, filter.GlobalStore("meta", meta))
	if err != nil {
		return fmt.Errorf("cannot parse filter: %s", filter.HumanError(err))
	}
	meta = &filter.Meta{ReverseDirection: true}
	reverse, err := filter.Parse("", input, filter.GlobalStore("meta", meta))
	if err != nil {
		return fmt.Errorf("cannot parse reverse filter: %s", filter.HumanError(err))
	}
	*qf = queryFilter{
		Filter:            direct.(string),
		ReverseFilter:     reverse.(string),
		MainTableRequired: meta.MainTableRequired,
	}
	return nil
}

// toSQLSelect transforms a column into an expression to use in SELECT
func (qc queryColumn) toSQLSelect() string {
	var strValue string
	switch schema.ColumnKey(qc) {
	case schema.ColumnExporterAddress, schema.ColumnSrcAddr, schema.ColumnDstAddr:
		strValue = fmt.Sprintf("replaceRegexpOne(IPv6NumToString(%s), '^::ffff:', '')", qc)
	case schema.ColumnSrcAS, schema.ColumnDstAS, schema.ColumnDst1stAS, schema.ColumnDst2ndAS, schema.ColumnDst3rdAS:
		strValue = fmt.Sprintf(`concat(toString(%s), ': ', dictGetOrDefault('asns', 'name', %s, '???'))`,
			qc, qc)
	case schema.ColumnEType:
		strValue = fmt.Sprintf(`if(EType = %d, 'IPv4', if(EType = %d, 'IPv6', '???'))`,
			helpers.ETypeIPv4, helpers.ETypeIPv6)
	case schema.ColumnProto:
		strValue = `dictGetOrDefault('protocols', 'name', Proto, '???')`
	case schema.ColumnInIfSpeed, schema.ColumnOutIfSpeed, schema.ColumnSrcPort, schema.ColumnDstPort, schema.ColumnForwardingStatus, schema.ColumnInIfBoundary, schema.ColumnOutIfBoundary:
		strValue = fmt.Sprintf("toString(%s)", qc)
	case schema.ColumnDstASPath:
		strValue = `arrayStringConcat(DstASPath, ' ')`
	case schema.ColumnDstCommunities:
		strValue = `arrayStringConcat(arrayConcat(arrayMap(c -> concat(toString(bitShiftRight(c, 16)), ':', toString(bitAnd(c, 0xffff))), DstCommunities), arrayMap(c -> concat(toString(bitAnd(bitShiftRight(c, 64), 0xffffffff)), ':', toString(bitAnd(bitShiftRight(c, 32), 0xffffffff)), ':', toString(bitAnd(c, 0xffffffff))), DstLargeCommunities)), ' ')`
	default:
		strValue = qc.String()
	}
	return strValue
}

// fixQueryColumnName fix capitalization of the provided column name
func fixQueryColumnName(name string) string {
	name = strings.ToLower(name)
	for pair := schema.Flows.Columns.Front(); pair != nil; pair = pair.Next() {
		column := pair.Value
		if strings.ToLower(column.Name) == name {
			return column.Name
		}
	}
	return ""
}
