package console

import (
	"errors"
	"fmt"

	"akvorado/common/helpers"
	"akvorado/console/filter"
)

type queryColumn int

const (
	queryColumnExporterAddress queryColumn = iota + 1
	queryColumnExporterName
	queryColumnExporterGroup
	queryColumnSrcAS
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
	queryColumnSrcAddr:           "SrcAddr",
	queryColumnDstAddr:           "DstAddr",
	queryColumnSrcAS:             "SrcAS",
	queryColumnDstAS:             "DstAS",
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
		strValue = `if(EType = 0x800, 'IPv4', if(EType = 0x86dd, 'IPv6', '???'))`
	case queryColumnProto:
		strValue = `dictGetOrDefault('protocols', 'name', Proto, '???')`
	case queryColumnInIfSpeed, queryColumnOutIfSpeed, queryColumnSrcPort, queryColumnDstPort, queryColumnForwardingStatus:
		strValue = fmt.Sprintf("toString(%s)", gc)
	default:
		strValue = gc.String()
	}
	return strValue
}
