// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strings"

	orderedmap "github.com/elliotchance/orderedmap/v2"
)

// Flows is the data schema for flows tables. Any column starting with Src/InIf
// will be duplicated as Dst/OutIf during init. That's not the case for columns
// in `PrimaryKeys'.
var Flows = Schema{
	ClickHousePrimaryKeys: []string{
		"TimeReceived",
		"ExporterAddress",
		"EType",
		"Proto",
		"InIfName",
		"SrcAS",
		"ForwardingStatus",
		"OutIfName",
		"DstAS",
		"SamplingRate",
	},
	Columns: buildMapFromColumns([]Column{
		{
			Name:                "TimeReceived",
			ClickHouseType:      "DateTime",
			ClickHouseCodec:     "DoubleDelta, LZ4",
			ConsoleNotDimension: true,
		},
		{Name: "SamplingRate", ClickHouseType: "UInt64", ConsoleNotDimension: true},
		{Name: "ExporterAddress", ClickHouseType: "LowCardinality(IPv6)"},
		{Name: "ExporterName", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "ExporterGroup", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "ExporterRole", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "ExporterSite", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "ExporterRegion", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "ExporterTenant", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{
			Name:           "SrcAddr",
			MainOnly:       true,
			ClickHouseType: "IPv6",
		}, {
			Name:                "SrcNetMask",
			MainOnly:            true,
			ClickHouseType:      "UInt8",
			ConsoleNotDimension: true,
		}, {
			Name:           "SrcNetPrefix",
			MainOnly:       true,
			ClickHouseType: "String",
			ClickHouseAlias: `CASE
 WHEN EType = 0x800 THEN concat(replaceRegexpOne(IPv6CIDRToRange(SrcAddr, (96 + SrcNetMask)::UInt8).1::String, '^::ffff:', ''), '/', SrcNetMask::String)
 WHEN EType = 0x86dd THEN concat(IPv6CIDRToRange(SrcAddr, SrcNetMask).1::String, '/', SrcNetMask::String)
 ELSE ''
END`,
		},
		{Name: "SrcAS", ClickHouseType: "UInt32"},
		{
			Name:                   "SrcNetName",
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'name', SrcAddr, '')",
		}, {
			Name:                   "SrcNetRole",
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'role', SrcAddr, '')",
		}, {
			Name:                   "SrcNetSite",
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'site', SrcAddr, '')",
		}, {
			Name:                   "SrcNetRegion",
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'region', SrcAddr, '')",
		}, {
			Name:                   "SrcNetTenant",
			ClickHouseType:         "LowCardinality(String)",
			ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'tenant', SrcAddr, '')",
		},
		{Name: "SrcCountry", ClickHouseType: "FixedString(2)"},
		{
			Name:           "DstASPath",
			MainOnly:       true,
			ClickHouseType: "Array(UInt32)",
		}, {
			Name:                   "Dst1stAS",
			ClickHouseType:         "UInt32",
			ClickHouseGenerateFrom: "c_DstASPath[1]",
		}, {
			Name:                   "Dst2ndAS",
			ClickHouseType:         "UInt32",
			ClickHouseGenerateFrom: "c_DstASPath[2]",
		}, {
			Name:                   "Dst3rdAS",
			ClickHouseType:         "UInt32",
			ClickHouseGenerateFrom: "c_DstASPath[3]",
		}, {
			Name:           "DstCommunities",
			MainOnly:       true,
			ClickHouseType: "Array(UInt32)",
		}, {
			Name:           "DstLargeCommunities",
			MainOnly:       true,
			ClickHouseType: "Array(UInt128)",
			ClickHouseTransformFrom: []Column{
				{Name: "DstLargeCommunities.ASN", ClickHouseType: "Array(UInt32)"},
				{Name: "DstLargeCommunities.LocalData1", ClickHouseType: "Array(UInt32)"},
				{Name: "DstLargeCommunities.LocalData2", ClickHouseType: "Array(UInt32)"},
			},
			ClickHouseTransformTo: "arrayMap((asn, l1, l2) -> ((bitShiftLeft(CAST(asn, 'UInt128'), 64) + bitShiftLeft(CAST(l1, 'UInt128'), 32)) + CAST(l2, 'UInt128')), `DstLargeCommunities.ASN`, `DstLargeCommunities.LocalData1`, `DstLargeCommunities.LocalData2`)",
			ConsoleNotDimension:   true,
		},
		{Name: "InIfName", ClickHouseType: "LowCardinality(String)"},
		{Name: "InIfDescription", ClickHouseType: "String", ClickHouseNotSortingKey: true},
		{Name: "InIfSpeed", ClickHouseType: "UInt32", ClickHouseNotSortingKey: true},
		{Name: "InIfConnectivity", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "InIfProvider", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
		{Name: "InIfBoundary", ClickHouseType: "Enum8('undefined' = 0, 'external' = 1, 'internal' = 2)", ClickHouseNotSortingKey: true},
		{Name: "EType", ClickHouseType: "UInt32"},
		{Name: "Proto", ClickHouseType: "UInt32"},
		{Name: "SrcPort", ClickHouseType: "UInt32", MainOnly: true},
		{Name: "Bytes", ClickHouseType: "UInt64", ClickHouseNotSortingKey: true, ConsoleNotDimension: true},
		{Name: "Packets", ClickHouseType: "UInt64", ClickHouseNotSortingKey: true, ConsoleNotDimension: true},
		{
			Name:                "PacketSize",
			ClickHouseType:      "UInt64",
			ClickHouseAlias:     "intDiv(Bytes, Packets)",
			ConsoleNotDimension: true,
		}, {
			Name:           "PacketSizeBucket",
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
		{Name: "ForwardingStatus", ClickHouseType: "UInt32"},
	}),
}

func buildMapFromColumns(columns []Column) *orderedmap.OrderedMap[string, Column] {
	omap := orderedmap.NewOrderedMap[string, Column]()
	for _, column := range columns {
		// Add non-main columns with an alias to NotSortingKey
		if !column.MainOnly && column.ClickHouseAlias != "" {
			column.ClickHouseNotSortingKey = true
		}
		omap.Set(column.Name, column)
		// Expand the schema Src → Dst and InIf → OutIf
		if strings.HasPrefix(column.Name, "Src") {
			column.Name = fmt.Sprintf("Dst%s", column.Name[3:])
			column.ClickHouseAlias = strings.ReplaceAll(column.ClickHouseAlias, "Src", "Dst")
			omap.Set(column.Name, column)
		} else if strings.HasPrefix(column.Name, "InIf") {
			column.Name = fmt.Sprintf("OutIf%s", column.Name[4:])
			column.ClickHouseAlias = strings.ReplaceAll(column.ClickHouseAlias, "InIf", "OutIf")
			omap.Set(column.Name, column)
		}
	}
	return omap
}

func init() {
	for _, key := range Flows.ClickHousePrimaryKeys {
		if column, ok := Flows.Columns.Get(key); !ok {
			panic(fmt.Sprintf("primary key %q not a column", key))
		} else {
			if column.ClickHouseNotSortingKey {
				panic(fmt.Sprintf("primary key %q is marked as a non-sorting key", key))
			}
		}
	}
}
