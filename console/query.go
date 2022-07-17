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

const (
	queryColumnExporterAddress queryColumn = iota + 1
	queryColumnExporterName
	queryColumnExporterGroup
	queryColumnExporterRole
	queryColumnExporterSite
	queryColumnExporterRegion
	queryColumnExporterTenant
	queryColumnSrcAS
	queryColumnSrcNetName
	queryColumnSrcNetRole
	queryColumnSrcNetSite
	queryColumnSrcNetRegion
	queryColumnSrcNetTenant
	queryColumnSrcCountry
	queryColumnInIfName
	queryColumnInIfDescription
	queryColumnInIfSpeed
	queryColumnInIfConnectivity
	queryColumnInIfProvider
	queryColumnInIfBoundary
	queryColumnEType
	queryColumnProto
	queryColumnSrcPort
	queryColumnSrcAddr
	queryColumnDstAS
	queryColumnDstNetName
	queryColumnDstNetRole
	queryColumnDstNetSite
	queryColumnDstNetRegion
	queryColumnDstNetTenant
	queryColumnDstCountry
	queryColumnOutIfName
	queryColumnOutIfDescription
	queryColumnOutIfSpeed
	queryColumnOutIfConnectivity
	queryColumnOutIfProvider
	queryColumnOutIfBoundary
	queryColumnDstAddr
	queryColumnDstPort
	queryColumnForwardingStatus
	queryColumnPacketSizeBucket
)

var queryColumnMap = helpers.NewBimap(map[queryColumn]string{
	queryColumnExporterAddress:   "ExporterAddress",
	queryColumnExporterName:      "ExporterName",
	queryColumnExporterGroup:     "ExporterGroup",
	queryColumnExporterRole:      "ExporterRole",
	queryColumnExporterSite:      "ExporterSite",
	queryColumnExporterRegion:    "ExporterRegion",
	queryColumnExporterTenant:    "ExporterTenant",
	queryColumnSrcAddr:           "SrcAddr",
	queryColumnDstAddr:           "DstAddr",
	queryColumnSrcAS:             "SrcAS",
	queryColumnDstAS:             "DstAS",
	queryColumnSrcNetName:        "SrcNetName",
	queryColumnDstNetName:        "DstNetName",
	queryColumnSrcNetRole:        "SrcNetRole",
	queryColumnDstNetRole:        "DstNetRole",
	queryColumnSrcNetSite:        "SrcNetSite",
	queryColumnDstNetSite:        "DstNetSite",
	queryColumnSrcNetRegion:      "SrcNetRegion",
	queryColumnDstNetRegion:      "DstNetRegion",
	queryColumnSrcNetTenant:      "SrcNetTenant",
	queryColumnDstNetTenant:      "DstNetTenant",
	queryColumnSrcCountry:        "SrcCountry",
	queryColumnDstCountry:        "DstCountry",
	queryColumnInIfName:          "InIfName",
	queryColumnOutIfName:         "OutIfName",
	queryColumnInIfDescription:   "InIfDescription",
	queryColumnOutIfDescription:  "OutIfDescription",
	queryColumnInIfSpeed:         "InIfSpeed",
	queryColumnOutIfSpeed:        "OutIfSpeed",
	queryColumnInIfConnectivity:  "InIfConnectivity",
	queryColumnOutIfConnectivity: "OutIfConnectivity",
	queryColumnInIfProvider:      "InIfProvider",
	queryColumnOutIfProvider:     "OutIfProvider",
	queryColumnInIfBoundary:      "InIfBoundary",
	queryColumnOutIfBoundary:     "OutIfBoundary",
	queryColumnEType:             "EType",
	queryColumnProto:             "Proto",
	queryColumnSrcPort:           "SrcPort",
	queryColumnDstPort:           "DstPort",
	queryColumnForwardingStatus:  "ForwardingStatus",
	queryColumnPacketSizeBucket:  "PacketSizeBucket",
})

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
	filter string
}

func (gf queryFilter) MarshalText() ([]byte, error) {
	return []byte(gf.filter), nil
}
func (gf *queryFilter) UnmarshalText(input []byte) error {
	if strings.TrimSpace(string(input)) == "" {
		*gf = queryFilter{""}
		return nil
	}
	got, err := filter.Parse("", input)
	if err != nil {
		return fmt.Errorf("cannot parse filter: %s", filter.HumanError(err))
	}
	*gf = queryFilter{got.(string)}
	return nil
}

// toSQLSelect transforms a column into an expression to use in SELECT
func (gc queryColumn) toSQLSelect() string {
	var strValue string
	switch gc {
	case queryColumnExporterAddress, queryColumnSrcAddr, queryColumnDstAddr:
		strValue = fmt.Sprintf("IPv6NumToString(%s)", gc)
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
