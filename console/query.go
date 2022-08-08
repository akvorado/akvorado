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

func (gc queryColumn) MarshalText() ([]byte, error) {
	got, ok := queryColumnMap.LoadValue(gc)
	if ok {
		return []byte(got), nil
	}
	return nil, errors.New("unknown field")
}
func (gc queryColumn) String() string {
	got, _ := queryColumnMap.LoadValue(gc)
	return got
}
func (gc *queryColumn) UnmarshalText(input []byte) error {
	got, ok := queryColumnMap.LoadKey(string(input))
	if ok {
		*gc = got
		return nil
	}
	return errors.New("unknown field")
}

type queryFilter struct {
	Filter            string
	ReverseFilter     string
	MainTableRequired bool
}

func (gf queryFilter) String() string {
	return gf.Filter
}
func (gf queryFilter) MarshalText() ([]byte, error) {
	return []byte(gf.Filter), nil
}
func (gf *queryFilter) UnmarshalText(input []byte) error {
	if strings.TrimSpace(string(input)) == "" {
		*gf = queryFilter{}
		return nil
	}
	meta := &filter.Meta{}
	direct, err := filter.Parse("", input, filter.GlobalStore("meta", meta))
	if err != nil {
		return fmt.Errorf("cannot parse filter: %s", filter.HumanError(err))
	}
	meta = &filter.Meta{ReverseDirection: true}
	reverse, err := filter.Parse("", input, filter.GlobalStore("meta", meta))
	*gf = queryFilter{
		Filter:            direct.(string),
		ReverseFilter:     reverse.(string),
		MainTableRequired: meta.MainTableRequired,
	}
	return nil
}

// toSQLSelect transforms a column into an expression to use in SELECT
func (gc queryColumn) toSQLSelect() string {
	var strValue string
	switch gc {
	case queryColumnExporterAddress, queryColumnSrcAddr, queryColumnDstAddr:
		strValue = fmt.Sprintf("replaceRegexpOne(IPv6NumToString(%s), '^::ffff:', '')", gc)
	case queryColumnSrcAS, queryColumnDstAS:
		strValue = fmt.Sprintf(`concat(toString(%s), ': ', dictGetOrDefault('asns', 'name', %s, '???'))`,
			gc, gc)
	case queryColumnEType:
		strValue = fmt.Sprintf(`if(EType = %d, 'IPv4', if(EType = %d, 'IPv6', '???'))`,
			helpers.ETypeIPv4, helpers.ETypeIPv6)
	case queryColumnProto:
		strValue = `dictGetOrDefault('protocols', 'name', Proto, '???')`
	case queryColumnInIfSpeed, queryColumnOutIfSpeed, queryColumnSrcPort, queryColumnDstPort, queryColumnForwardingStatus, queryColumnInIfBoundary, queryColumnOutIfBoundary:
		strValue = fmt.Sprintf("toString(%s)", gc)
	default:
		strValue = gc.String()
	}
	return strValue
}

// reverseDirection reverse the direction of a column (src/dst, in/out)
func (gc queryColumn) reverseDirection() queryColumn {
	value, ok := queryColumnMap.LoadKey(filter.ReverseColumnDirection(gc.String()))
	if !ok {
		panic("unknown reverse column")
	}
	return value
}
