// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strings"

	"akvorado/common/helpers/bimap"

	orderedmap "github.com/elliotchance/orderedmap/v2"
)

// revive:disable
const (
	ColumnTimeReceived ColumnKey = iota + 1
	ColumnSamplingRate
	ColumnExporterAddress
	ColumnExporterName
	ColumnExporterGroup
	ColumnExporterRole
	ColumnExporterSite
	ColumnExporterRegion
	ColumnExporterTenant
	ColumnSrcAddr
	ColumnDstAddr
	ColumnSrcNetMask
	ColumnDstNetMask
	ColumnSrcNetPrefix
	ColumnDstNetPrefix
	ColumnSrcAS
	ColumnDstAS
	ColumnSrcNetName
	ColumnDstNetName
	ColumnSrcNetRole
	ColumnDstNetRole
	ColumnSrcNetSite
	ColumnDstNetSite
	ColumnSrcNetRegion
	ColumnDstNetRegion
	ColumnSrcNetTenant
	ColumnDstNetTenant
	ColumnSrcCountry
	ColumnDstCountry
	ColumnDstASPath
	ColumnDst1stAS
	ColumnDst2ndAS
	ColumnDst3rdAS
	ColumnDstCommunities
	ColumnDstLargeCommunities
	ColumnDstLargeCommunitiesASN
	ColumnDstLargeCommunitiesLocalData1
	ColumnDstLargeCommunitiesLocalData2
	ColumnInIfName
	ColumnOutIfName
	ColumnInIfDescription
	ColumnOutIfDescription
	ColumnInIfSpeed
	ColumnOutIfSpeed
	ColumnInIfProvider
	ColumnOutIfProvider
	ColumnInIfConnectivity
	ColumnOutIfConnectivity
	ColumnInIfBoundary
	ColumnOutIfBoundary
	ColumnEType
	ColumnProto
	ColumnSrcPort
	ColumnDstPort
	ColumnBytes
	ColumnPackets
	ColumnPacketSize
	ColumnPacketSizeBucket
	ColumnForwardingStatus
)

// revive:enable

var columnNameMap = bimap.New(map[ColumnKey]string{
	ColumnTimeReceived:                  "TimeReceived",
	ColumnSamplingRate:                  "SamplingRate",
	ColumnExporterAddress:               "ExporterAddress",
	ColumnExporterName:                  "ExporterName",
	ColumnExporterGroup:                 "ExporterGroup",
	ColumnExporterRole:                  "ExporterRole",
	ColumnExporterSite:                  "ExporterSite",
	ColumnExporterRegion:                "ExporterRegion",
	ColumnExporterTenant:                "ExporterTenant",
	ColumnSrcAddr:                       "SrcAddr",
	ColumnDstAddr:                       "DstAddr",
	ColumnSrcNetMask:                    "SrcNetMask",
	ColumnDstNetMask:                    "DstNetMask",
	ColumnSrcNetPrefix:                  "SrcNetPrefix",
	ColumnDstNetPrefix:                  "DstNetPrefix",
	ColumnSrcAS:                         "SrcAS",
	ColumnDstAS:                         "DstAS",
	ColumnSrcNetName:                    "SrcNetName",
	ColumnDstNetName:                    "DstNetName",
	ColumnSrcNetRole:                    "SrcNetRole",
	ColumnDstNetRole:                    "DstNetRole",
	ColumnSrcNetSite:                    "SrcNetSite",
	ColumnDstNetSite:                    "DstNetSite",
	ColumnSrcNetRegion:                  "SrcNetRegion",
	ColumnDstNetRegion:                  "DstNetRegion",
	ColumnSrcNetTenant:                  "SrcNetTenant",
	ColumnDstNetTenant:                  "DstNetTenant",
	ColumnSrcCountry:                    "SrcCountry",
	ColumnDstCountry:                    "DstCountry",
	ColumnDstASPath:                     "DstASPath",
	ColumnDst1stAS:                      "Dst1stAS",
	ColumnDst2ndAS:                      "Dst2ndAS",
	ColumnDst3rdAS:                      "Dst3rdAS",
	ColumnDstCommunities:                "DstCommunities",
	ColumnDstLargeCommunities:           "DstLargeCommunities",
	ColumnDstLargeCommunitiesASN:        "DstLargeCommunities.ASN",
	ColumnDstLargeCommunitiesLocalData1: "DstLargeCommunities.LocalData1",
	ColumnDstLargeCommunitiesLocalData2: "DstLargeCommunities.LocalData2",
	ColumnInIfName:                      "InIfName",
	ColumnOutIfName:                     "OutIfName",
	ColumnInIfDescription:               "InIfDescription",
	ColumnOutIfDescription:              "OutIfDescription",
	ColumnInIfSpeed:                     "InIfSpeed",
	ColumnOutIfSpeed:                    "OutIfSpeed",
	ColumnInIfProvider:                  "InIfProvider",
	ColumnOutIfProvider:                 "OutIfProvider",
	ColumnInIfConnectivity:              "InIfConnectivity",
	ColumnOutIfConnectivity:             "OutIfConnectivity",
	ColumnInIfBoundary:                  "InIfBoundary",
	ColumnOutIfBoundary:                 "OutIfBoundary",
	ColumnEType:                         "EType",
	ColumnProto:                         "Proto",
	ColumnSrcPort:                       "SrcPort",
	ColumnDstPort:                       "DstPort",
	ColumnBytes:                         "Bytes",
	ColumnPackets:                       "Packets",
	ColumnPacketSize:                    "PacketSize",
	ColumnPacketSizeBucket:              "PacketSizeBucket",
	ColumnForwardingStatus:              "ForwardingStatus",
})

func (c ColumnKey) String() string {
	name, _ := columnNameMap.LoadValue(c)
	return name
}

// Flows is the data schema for flows tables. Any column starting with Src/InIf
// will be duplicated as Dst/OutIf during init. That's not the case for columns
// in `PrimaryKeys'.
var Flows = Schema{
	clickHousePrimaryKeys: []ColumnKey{
		ColumnTimeReceived,
		ColumnExporterAddress,
		ColumnEType,
		ColumnProto,
		ColumnInIfName,
		ColumnSrcAS,
		ColumnForwardingStatus,
		ColumnOutIfName,
		ColumnDstAS,
		ColumnSamplingRate,
	},
	columns: buildMapFromColumns([]Column{
		{
			Key:                 ColumnTimeReceived,
			ClickHouseType:      "DateTime",
			ClickHouseCodec:     "DoubleDelta, LZ4",
			ConsoleNotDimension: true,
		},
		{Key: ColumnSamplingRate, ClickHouseType: "UInt64", ConsoleNotDimension: true},
		{Key: ColumnExporterAddress, ClickHouseType: "LowCardinality(IPv6)"},
		{Key: ColumnExporterName, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnExporterGroup, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnExporterRole, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnExporterSite, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnExporterRegion, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnExporterTenant, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{
			Key:            ColumnSrcAddr,
			MainOnly:       true,
			ClickHouseType: "IPv6",
		}, {
			Key:                 ColumnSrcNetMask,
			MainOnly:            true,
			ClickHouseType:      "UInt8",
			ConsoleNotDimension: true,
		}, {
			Key:            ColumnSrcNetPrefix,
			MainOnly:       true,
			ClickHouseType: "String",
			ClickHouseAlias: `CASE
 WHEN EType = 0x800 THEN concat(replaceRegexpOne(IPv6CIDRToRange(SrcAddr, (96 + SrcNetMask)::UInt8).1::String, '^::ffff:', ''), '/', SrcNetMask::String)
 WHEN EType = 0x86dd THEN concat(IPv6CIDRToRange(SrcAddr, SrcNetMask).1::String, '/', SrcNetMask::String)
 ELSE ''
END`,
		},
		{Key: ColumnSrcAS, ClickHouseType: "UInt32"},
		{
			Key:                    ColumnSrcNetName,
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'name', SrcAddr, '')",
		}, {
			Key:                    ColumnSrcNetRole,
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'role', SrcAddr, '')",
		}, {
			Key:                    ColumnSrcNetSite,
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'site', SrcAddr, '')",
		}, {
			Key:                    ColumnSrcNetRegion,
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'region', SrcAddr, '')",
		}, {
			Key:                    ColumnSrcNetTenant,
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'tenant', SrcAddr, '')",
		},
		{Key: ColumnSrcCountry, ClickHouseType: "FixedString(2)"},
		{
			Key:            ColumnDstASPath,
			MainOnly:       true,
			ClickHouseType: "Array(UInt32)",
		}, {
			Key:                    ColumnDst1stAS,
			ClickHouseType:         "UInt32",
			ClickHouseGenerateFrom: "c_DstASPath[1]",
		}, {
			Key:                    ColumnDst2ndAS,
			ClickHouseType:         "UInt32",
			ClickHouseGenerateFrom: "c_DstASPath[2]",
		}, {
			Key:                    ColumnDst3rdAS,
			ClickHouseType:         "UInt32",
			ClickHouseGenerateFrom: "c_DstASPath[3]",
		}, {
			Key:            ColumnDstCommunities,
			MainOnly:       true,
			ClickHouseType: "Array(UInt32)",
		}, {
			Key:            ColumnDstLargeCommunities,
			MainOnly:       true,
			ClickHouseType: "Array(UInt128)",
			ClickHouseTransformFrom: []Column{
				{Key: ColumnDstLargeCommunitiesASN, ClickHouseType: "Array(UInt32)"},
				{Key: ColumnDstLargeCommunitiesLocalData1, ClickHouseType: "Array(UInt32)"},
				{Key: ColumnDstLargeCommunitiesLocalData2, ClickHouseType: "Array(UInt32)"},
			},
			ClickHouseTransformTo: "arrayMap((asn, l1, l2) -> ((bitShiftLeft(CAST(asn, 'UInt128'), 64) + bitShiftLeft(CAST(l1, 'UInt128'), 32)) + CAST(l2, 'UInt128')), `DstLargeCommunities.ASN`, `DstLargeCommunities.LocalData1`, `DstLargeCommunities.LocalData2`)",
			ConsoleNotDimension:   true,
		},
		{Key: ColumnInIfName, ClickHouseType: "LowCardinality(String)"},
		{Key: ColumnInIfDescription, ClickHouseType: "String", ClickHouseNotSortingKey: true},
		{Key: ColumnInIfSpeed, ClickHouseType: "UInt32", ClickHouseNotSortingKey: true},
		{Key: ColumnInIfConnectivity, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnInIfProvider, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Key: ColumnInIfBoundary, ClickHouseType: "Enum8('undefined' = 0, 'external' = 1, 'internal' = 2)", ClickHouseNotSortingKey: true},
		{Key: ColumnEType, ClickHouseType: "UInt32"},
		{Key: ColumnProto, ClickHouseType: "UInt32"},
		{Key: ColumnSrcPort, ClickHouseType: "UInt32", MainOnly: true},
		{Key: ColumnBytes, ClickHouseType: "UInt64", ClickHouseNotSortingKey: true, ConsoleNotDimension: true},
		{Key: ColumnPackets, ClickHouseType: "UInt64", ClickHouseNotSortingKey: true, ConsoleNotDimension: true},
		{
			Key:                 ColumnPacketSize,
			ClickHouseType:      "UInt64",
			ClickHouseAlias:     "intDiv(Bytes, Packets)",
			ConsoleNotDimension: true,
		}, {
			Key:            ColumnPacketSizeBucket,
			ClickHouseType: "LowCardinality(String)",
			ClickHouseAlias: func() string {
				boundaries := []int{64, 128, 256, 512, 768, 1024, 1280, 1501,
					2048, 3072, 4096, 8192, 10240, 16384, 32768, 65536}
				conditions := []string{}
				last := 0
				for _, boundary := range boundaries {
					conditions = append(conditions, fmt.Sprintf("PacketSize < %d, '%d-%d'",
						boundary, last, boundary-1))
					last = boundary
				}
				conditions = append(conditions, fmt.Sprintf("'%d-Inf'", last))
				return fmt.Sprintf("multiIf(%s)", strings.Join(conditions, ", "))
			}(),
		},
		{Key: ColumnForwardingStatus, ClickHouseType: "UInt32"},
	}),
}

func buildMapFromColumns(columns []Column) *orderedmap.OrderedMap[ColumnKey, Column] {
	omap := orderedmap.NewOrderedMap[ColumnKey, Column]()
	for _, column := range columns {
		// Add true name
		name, ok := columnNameMap.LoadValue(column.Key)
		if !ok {
			panic(fmt.Sprintf("missing name mapping for %d", column.Key))
		}
		column.Name = name

		// Also true name for columns in ClickHouseTransformFrom
		for idx, ecolumn := range column.ClickHouseTransformFrom {
			name, ok := columnNameMap.LoadValue(ecolumn.Key)
			if !ok {
				panic(fmt.Sprintf("missing name mapping for %d", ecolumn.Key))
			}
			column.ClickHouseTransformFrom[idx].Name = name
		}

		// Add non-main columns with an alias to NotSortingKey
		if !column.MainOnly && column.ClickHouseAlias != "" {
			column.ClickHouseNotSortingKey = true
		}
		omap.Set(column.Key, column)

		// Expand the schema Src → Dst and InIf → OutIf
		if strings.HasPrefix(name, "Src") {
			column.Name = fmt.Sprintf("Dst%s", name[3:])
			column.Key, ok = columnNameMap.LoadKey(column.Name)
			if !ok {
				panic(fmt.Sprintf("missing name mapping for %q", column.Name))
			}
			column.ClickHouseAlias = strings.ReplaceAll(column.ClickHouseAlias, "Src", "Dst")
			omap.Set(column.Key, column)
		} else if strings.HasPrefix(name, "InIf") {
			column.Name = fmt.Sprintf("OutIf%s", name[4:])
			column.Key, ok = columnNameMap.LoadKey(column.Name)
			if !ok {
				panic(fmt.Sprintf("missing name mapping for %q", column.Name))
			}
			column.ClickHouseAlias = strings.ReplaceAll(column.ClickHouseAlias, "InIf", "OutIf")
			omap.Set(column.Key, column)
		}
	}
	return omap
}

func init() {
	for _, key := range Flows.clickHousePrimaryKeys {
		if column, ok := Flows.columns.Get(key); !ok {
			panic(fmt.Sprintf("primary key %q not a column", key))
		} else {
			if column.ClickHouseNotSortingKey {
				panic(fmt.Sprintf("primary key %q is marked as a non-sorting key", key))
			}
		}
	}
}
