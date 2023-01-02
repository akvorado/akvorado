// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strings"
)

// Flows is the data schema for flows tables. Any column starting with Src/InIf
// will be duplicated as Dst/OutIf during init. That's not the case for columns
// in `PrimaryKeys'.
var Flows = Schema{
	PrimaryKeys: []string{
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
	Columns: []Column{
		{
			Name:  "TimeReceived",
			Type:  "DateTime",
			Codec: "DoubleDelta, LZ4",
		},
		{Name: "SamplingRate", Type: "UInt64"},
		{Name: "ExporterAddress", Type: "LowCardinality(IPv6)"},
		{Name: "ExporterName", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "ExporterGroup", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "ExporterRole", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "ExporterSite", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "ExporterRegion", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "ExporterTenant", Type: "LowCardinality(String)", NotSortingKey: true},
		{
			Name:     "SrcAddr",
			Type:     "IPv6",
			MainOnly: true,
		}, {
			Name:     "SrcNetMask",
			Type:     "UInt8",
			MainOnly: true,
		}, {
			Name:     "SrcNetPrefix",
			Type:     "String",
			MainOnly: true,
			Alias: `CASE
 WHEN EType = 0x800 THEN concat(replaceRegexpOne(IPv6CIDRToRange(SrcAddr, (96 + SrcNetMask)::UInt8).1::String, '^::ffff:', ''), '/', SrcNetMask::String)
 WHEN EType = 0x86dd THEN concat(IPv6CIDRToRange(SrcAddr, SrcNetMask).1::String, '/', SrcNetMask::String)
 ELSE ''
END`,
		},
		{Name: "SrcAS", Type: "UInt32"},
		{
			Name:         "SrcNetName",
			Type:         "LowCardinality(String)",
			GenerateFrom: "dictGetOrDefault('networks', 'name', SrcAddr, '')",
		}, {
			Name:         "SrcNetRole",
			Type:         "LowCardinality(String)",
			GenerateFrom: "dictGetOrDefault('networks', 'role', SrcAddr, '')",
		}, {
			Name:         "SrcNetSite",
			Type:         "LowCardinality(String)",
			GenerateFrom: "dictGetOrDefault('networks', 'site', SrcAddr, '')",
		}, {
			Name:         "SrcNetRegion",
			Type:         "LowCardinality(String)",
			GenerateFrom: "dictGetOrDefault('networks', 'region', SrcAddr, '')",
		}, {
			Name:         "SrcNetTenant",
			Type:         "LowCardinality(String)",
			GenerateFrom: "dictGetOrDefault('networks', 'tenant', SrcAddr, '')",
		},
		{Name: "SrcCountry", Type: "FixedString(2)"},
		{
			Name:     "DstASPath",
			Type:     "Array(UInt32)",
			MainOnly: true,
		}, {
			Name:         "Dst1stAS",
			Type:         "UInt32",
			GenerateFrom: "c_DstASPath[1]",
		}, {
			Name:         "Dst2ndAS",
			Type:         "UInt32",
			GenerateFrom: "c_DstASPath[2]",
		}, {
			Name:         "Dst3rdAS",
			Type:         "UInt32",
			GenerateFrom: "c_DstASPath[3]",
		}, {
			Name:     "DstCommunities",
			Type:     "Array(UInt32)",
			MainOnly: true,
		}, {
			Name:     "DstLargeCommunities",
			Type:     "Array(UInt128)",
			MainOnly: true,
			TransformFrom: []Column{
				{Name: "DstLargeCommunities.ASN", Type: "Array(UInt32)"},
				{Name: "DstLargeCommunities.LocalData1", Type: "Array(UInt32)"},
				{Name: "DstLargeCommunities.LocalData2", Type: "Array(UInt32)"},
			},
			TransformTo: "arrayMap((asn, l1, l2) -> ((bitShiftLeft(CAST(asn, 'UInt128'), 64) + bitShiftLeft(CAST(l1, 'UInt128'), 32)) + CAST(l2, 'UInt128')), `DstLargeCommunities.ASN`, `DstLargeCommunities.LocalData1`, `DstLargeCommunities.LocalData2`)",
		},
		{Name: "InIfName", Type: "LowCardinality(String)"},
		{Name: "InIfDescription", Type: "String", NotSortingKey: true},
		{Name: "InIfSpeed", Type: "UInt32", NotSortingKey: true},
		{Name: "InIfConnectivity", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "InIfProvider", Type: "LowCardinality(String)", NotSortingKey: true},
		{Name: "InIfBoundary", Type: "Enum8('undefined' = 0, 'external' = 1, 'internal' = 2)", NotSortingKey: true},
		{Name: "EType", Type: "UInt32"},
		{Name: "Proto", Type: "UInt32"},
		{Name: "SrcPort", Type: "UInt32", MainOnly: true},
		{Name: "Bytes", Type: "UInt64", NotSortingKey: true},
		{Name: "Packets", Type: "UInt64", NotSortingKey: true},
		{
			Name:  "PacketSize",
			Type:  "UInt64",
			Alias: "intDiv(Bytes, Packets)",
		}, {
			Name: "PacketSizeBucket",
			Type: "LowCardinality(String)",
			Alias: func() string {
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
		{Name: "ForwardingStatus", Type: "UInt32"},
	},
}

func init() {
	// Expand the schema Src → Dst and InIf → OutIf
	newSchema := []Column{}
	for _, column := range Flows.Columns {
		newSchema = append(newSchema, column)
		if strings.HasPrefix(column.Name, "Src") {
			column.Name = fmt.Sprintf("Dst%s", column.Name[3:])
			column.Alias = strings.ReplaceAll(column.Alias, "Src", "Dst")
			newSchema = append(newSchema, column)
		} else if strings.HasPrefix(column.Name, "InIf") {
			column.Name = fmt.Sprintf("OutIf%s", column.Name[4:])
			column.Alias = strings.ReplaceAll(column.Alias, "InIf", "OutIf")
			newSchema = append(newSchema, column)
		}
	}
	Flows.Columns = newSchema

	// Add non-main columns with an alias to NotSortingKey
	for idx, column := range Flows.Columns {
		if !column.MainOnly && column.Alias != "" {
			Flows.Columns[idx].NotSortingKey = true
		}
	}

	// Also do some checks.
outer:
	for _, key := range Flows.PrimaryKeys {
		for _, column := range Flows.Columns {
			if column.Name == key {
				if column.NotSortingKey {
					panic(fmt.Sprintf("primary key %q is marked as a non-sorting key", key))
				}
				continue outer
			}
		}
		panic(fmt.Sprintf("primary key %q not a column", key))
	}
}
