// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"errors"
	"fmt"
	"strings"

	"akvorado/common/helpers"
	"akvorado/console/filter"
)

type queryColumn int

func (qc queryColumn) MarshalText() ([]byte, error) {
	got, ok := queryColumnMap.LoadValue(qc)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown field")
}
func (qc queryColumn) String() string {
	got, _ := queryColumnMap.LoadValue(qc)
	return got
}
func (qc *queryColumn) UnmarshalText(input []byte) error {
	got, ok := queryColumnMap.LoadKey(string(input))
	if ok {
		*qc = got
		return nil
	}
	return errors.New("unknown field")
}

// queryColumnsRequiringMainTable lists query columns only present in
// the main table. Also check filter/parser.peg.
var queryColumnsRequiringMainTable = map[queryColumn]struct{}{
	queryColumnSrcAddr: {},
	queryColumnDstAddr: {},
	queryColumnSrcPort: {},
	queryColumnDstPort: {},
}

func requireMainTable(qcs []queryColumn, qf queryFilter) bool {
	if qf.MainTableRequired {
		return true
	}
	for _, qc := range qcs {
		if _, ok := queryColumnsRequiringMainTable[qc]; ok {
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
	switch qc {
	case queryColumnExporterAddress, queryColumnSrcAddr, queryColumnDstAddr:
		strValue = fmt.Sprintf("replaceRegexpOne(IPv6NumToString(%s), '^::ffff:', '')", qc)
	case queryColumnSrcAS, queryColumnDstAS:
		strValue = fmt.Sprintf(`concat(toString(%s), ': ', dictGetOrDefault('asns', 'name', %s, '???'))`,
			qc, qc)
	case queryColumnEType:
		strValue = fmt.Sprintf(`if(EType = %d, 'IPv4', if(EType = %d, 'IPv6', '???'))`,
			helpers.ETypeIPv4, helpers.ETypeIPv6)
	case queryColumnProto:
		strValue = `dictGetOrDefault('protocols', 'name', Proto, '???')`
	case queryColumnInIfSpeed, queryColumnOutIfSpeed, queryColumnSrcPort, queryColumnDstPort, queryColumnForwardingStatus, queryColumnInIfBoundary, queryColumnOutIfBoundary:
		strValue = fmt.Sprintf("toString(%s)", qc)
	default:
		strValue = qc.String()
	}
	return strValue
}

// reverseDirection reverse the direction of a column (src/dst, in/out)
func (qc queryColumn) reverseDirection() queryColumn {
	value, ok := queryColumnMap.LoadKey(filter.ReverseColumnDirection(qc.String()))
	if !ok {
		panic("unknown reverse column")
	}
	return value
}
