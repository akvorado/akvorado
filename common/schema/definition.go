// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strings"

	"github.com/bits-and-blooms/bitset"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// revive:disable
const (
	ColumnTimeReceived ColumnKey = iota + 1
	ColumnSamplingRate
	ColumnEType
	ColumnProto
	ColumnBytes
	ColumnPackets
	ColumnPacketSize
	ColumnPacketSizeBucket
	ColumnForwardingStatus
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
	ColumnSrcVlan
	ColumnDstVlan
	ColumnSrcPort
	ColumnDstPort
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
	ColumnSrcAddrNAT
	ColumnDstAddrNAT
	ColumnSrcPortNAT
	ColumnDstPortNAT
	ColumnSrcMAC
	ColumnDstMAC
	ColumnIPTTL
	ColumnIPTos
	ColumnIPFragmentID
	ColumnIPFragmentOffset
	ColumnIPv6FlowLabel
	ColumnTCPFlags
	ColumnICMPv4
	ColumnICMPv4Type
	ColumnICMPv4Code
	ColumnICMPv6
	ColumnICMPv6Type
	ColumnICMPv6Code
	ColumnNextHop

	// ColumnLast points to after the last static column, custom dictionaries (dynamic columns) come after ColumnLast
	ColumnLast
)

const (
	ColumnGroupL2 ColumnGroup = iota + 1
	ColumnGroupNAT
	ColumnGroupL3L4

	ColumnGroupLast
)

// revive:enable

// Flows is the data schema for flows tables. Any column starting with Src/InIf
// will be duplicated as Dst/OutIf during init. That's not the case for columns
// in `PrimaryKeys'.
func flows() Schema {
	return Schema{
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
		columns: []Column{
			{
				Key:                 ColumnTimeReceived,
				NoDisable:           true,
				ClickHouseType:      "DateTime",
				ClickHouseCodec:     "DoubleDelta, LZ4",
				ConsoleNotDimension: true,
				ProtobufType:        protoreflect.Uint64Kind,
			},
			{Key: ColumnSamplingRate, NoDisable: true, ClickHouseType: "UInt64", ConsoleNotDimension: true},
			{Key: ColumnExporterAddress, ClickHouseType: "LowCardinality(IPv6)"},
			{Key: ColumnExporterName, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterGroup, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterRole, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterSite, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterRegion, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterTenant, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{
				Key:                ColumnSrcAddr,
				ClickHouseMainOnly: true,
				ClickHouseType:     "IPv6",
				ClickHouseCodec:    "ZSTD(1)",
				ConsoleTruncateIP:  true,
			},
			{
				Key:                 ColumnSrcNetMask,
				ClickHouseMainOnly:  true,
				ClickHouseType:      "UInt8",
				ConsoleNotDimension: true,
			},
			{
				Key:                ColumnSrcNetPrefix,
				ClickHouseMainOnly: true,
				ClickHouseType:     "String",
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
			},
			{
				Key:                    ColumnDstNetName,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'name', DstAddr, '')",
			},
			{
				Key:                    ColumnSrcNetRole,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'role', SrcAddr, '')",
			},
			{
				Key:                    ColumnDstNetRole,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'role', DstAddr, '')",
			},
			{
				Key:                    ColumnSrcNetSite,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'site', SrcAddr, '')",
			},
			{
				Key:                    ColumnDstNetSite,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'site', DstAddr, '')",
			},
			{
				Key:                    ColumnSrcNetRegion,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'region', SrcAddr, '')",
			},
			{
				Key:                    ColumnDstNetRegion,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'region', DstAddr, '')",
			},
			{
				Key:                    ColumnSrcNetTenant,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'tenant', SrcAddr, '')",
			},
			{
				Key:                    ColumnDstNetTenant,
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "dictGetOrDefault('networks', 'tenant', DstAddr, '')",
			},
			{Key: ColumnSrcVlan, ClickHouseType: "UInt16", Disabled: true, Group: ColumnGroupL2},
			{Key: ColumnSrcCountry, ClickHouseType: "FixedString(2)"},
			{
				Key:                ColumnDstASPath,
				ClickHouseMainOnly: true,
				ClickHouseType:     "Array(UInt32)",
			},
			{
				Key:                    ColumnDst1stAS,
				Depends:                []ColumnKey{ColumnDstASPath},
				ClickHouseType:         "UInt32",
				ClickHouseGenerateFrom: "c_DstASPath[1]",
			},
			{
				Key:                    ColumnDst2ndAS,
				Depends:                []ColumnKey{ColumnDstASPath},
				ClickHouseType:         "UInt32",
				ClickHouseGenerateFrom: "c_DstASPath[2]",
			},
			{
				Key:                    ColumnDst3rdAS,
				Depends:                []ColumnKey{ColumnDstASPath},
				ClickHouseType:         "UInt32",
				ClickHouseGenerateFrom: "c_DstASPath[3]",
			},
			{
				Key:                ColumnDstCommunities,
				ClickHouseMainOnly: true,
				ClickHouseType:     "Array(UInt32)",
			},
			{
				Key:                ColumnDstLargeCommunities,
				ClickHouseMainOnly: true,
				ClickHouseType:     "Array(UInt128)",
				ClickHouseTransformFrom: []Column{
					{
						Key:            ColumnDstLargeCommunitiesASN,
						ClickHouseType: "Array(UInt32)",
					},
					{
						Key:            ColumnDstLargeCommunitiesLocalData1,
						ClickHouseType: "Array(UInt32)",
					},
					{
						Key:            ColumnDstLargeCommunitiesLocalData2,
						ClickHouseType: "Array(UInt32)",
					},
				},
				ClickHouseTransformTo: "arrayMap((asn, l1, l2) -> ((bitShiftLeft(CAST(asn, 'UInt128'), 64) + bitShiftLeft(CAST(l1, 'UInt128'), 32)) + CAST(l2, 'UInt128')), DstLargeCommunitiesASN, DstLargeCommunitiesLocalData1, DstLargeCommunitiesLocalData2)",
				ConsoleNotDimension:   true,
			},
			{Key: ColumnInIfName, ClickHouseType: "LowCardinality(String)"},
			{Key: ColumnInIfDescription, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnInIfSpeed, ClickHouseType: "UInt32", ClickHouseNotSortingKey: true},
			{Key: ColumnInIfConnectivity, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnInIfProvider, ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{
				Key:                     ColumnInIfBoundary,
				ClickHouseType:          "Enum8('undefined' = 0, 'external' = 1, 'internal' = 2)",
				ClickHouseNotSortingKey: true,
				ProtobufType:            protoreflect.EnumKind,
				ProtobufEnumName:        "Boundary",
				ProtobufEnum: map[int]string{
					0: "UNDEFINED",
					1: "EXTERNAL",
					2: "INTERNAL",
				},
			},
			{Key: ColumnEType, ClickHouseType: "UInt32"}, // TODO: UInt16 but hard to change, primary key
			{Key: ColumnProto, ClickHouseType: "UInt32"}, // TODO: UInt8 but hard to change, primary key
			{Key: ColumnSrcPort, ClickHouseType: "UInt16", ClickHouseMainOnly: true},
			{
				Key:                     ColumnBytes,
				NoDisable:               true,
				ClickHouseType:          "UInt64",
				ClickHouseCodec:         "T64, LZ4",
				ClickHouseNotSortingKey: true,
				ConsoleNotDimension:     true,
			},
			{
				Key:                     ColumnPackets,
				NoDisable:               true,
				ClickHouseType:          "UInt64",
				ClickHouseCodec:         "T64, LZ4",
				ClickHouseNotSortingKey: true,
				ConsoleNotDimension:     true,
			},
			{
				Key:                 ColumnPacketSize,
				Depends:             []ColumnKey{ColumnBytes, ColumnPackets},
				ClickHouseType:      "UInt64",
				ClickHouseAlias:     "intDiv(Bytes, Packets)",
				ConsoleNotDimension: true,
			},
			{
				Key:            ColumnPacketSizeBucket,
				Depends:        []ColumnKey{ColumnPacketSize},
				ClickHouseType: "LowCardinality(String)",
				ClickHouseAlias: func() string {
					boundaries := []int{
						64, 128, 256, 512, 768, 1024, 1280, 1501,
						2048, 3072, 4096, 8192, 10240, 16384, 32768, 65536,
					}
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
			{Key: ColumnForwardingStatus, ClickHouseType: "UInt32"}, // TODO: UInt8 but hard to change, primary key
			{
				Key:                ColumnSrcAddrNAT,
				Disabled:           true,
				Group:              ColumnGroupNAT,
				ClickHouseType:     "IPv6",
				ClickHouseMainOnly: true,
				ConsoleTruncateIP:  true,
			},
			{
				Key:                ColumnSrcPortNAT,
				Disabled:           true,
				Group:              ColumnGroupNAT,
				ClickHouseType:     "UInt16",
				ClickHouseMainOnly: true,
			},
			{Key: ColumnSrcMAC, Disabled: true, Group: ColumnGroupL2, ClickHouseType: "UInt64"},
			{Key: ColumnIPTTL, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt8"},
			{Key: ColumnIPTos, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt8"},
			{Key: ColumnIPFragmentID, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt32"},
			{Key: ColumnIPFragmentOffset, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt16"},
			{Key: ColumnIPv6FlowLabel, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt32"},
			{Key: ColumnTCPFlags, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt16"},
			{Key: ColumnICMPv4Type, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt8"},
			{Key: ColumnICMPv4Code, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt8"},
			{Key: ColumnICMPv6Type, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt8"},
			{Key: ColumnICMPv6Code, Disabled: true, Group: ColumnGroupL3L4, ClickHouseType: "UInt8"},
			{
				Key:            ColumnICMPv4,
				Depends:        []ColumnKey{ColumnProto, ColumnICMPv4Type, ColumnICMPv4Code},
				Disabled:       true,
				Group:          ColumnGroupL3L4,
				ClickHouseType: "LowCardinality(String)",
				ClickHouseAlias: `if(Proto = 1, ` +
					`dictGetOrDefault('icmp', 'name', tuple(Proto, ICMPv4Type, ICMPv4Code), ` +
					`concat(toString(ICMPv4Type), '/', toString(ICMPv4Code))), '')`,
			},
			{
				Key:            ColumnICMPv6,
				Depends:        []ColumnKey{ColumnProto, ColumnICMPv6Type, ColumnICMPv6Code},
				Disabled:       true,
				Group:          ColumnGroupL3L4,
				ClickHouseType: "LowCardinality(String)",
				ClickHouseAlias: `if(Proto = 58, ` +
					`dictGetOrDefault('icmp', 'name', tuple(Proto, ICMPv6Type, ICMPv6Code), ` +
					`concat(toString(ICMPv6Type), '/', toString(ICMPv6Code))), '')`,
			},
			{
				Key:             ColumnNextHop,
				Disabled:        true,
				ClickHouseType:  "LowCardinality(IPv6)",
				ClickHouseCodec: "ZSTD(1)",
			},
		},
		dynamicColumns: 0,
	}.finalize()
}

func (schema Schema) finalize() Schema {
	ncolumns := []Column{}
	for _, column := range schema.columns {
		// Add true name
		name, ok := columnNameMap.LoadValue(column.Key)
		if !ok {
			panic(fmt.Sprintf("missing name mapping for %d", column.Key))
		}
		if column.Name == "" {
			column.Name = name
		}

		// Also true name for columns in ClickHouseTransformFrom
		for idx, ecolumn := range column.ClickHouseTransformFrom {
			if ecolumn.Name == "" {
				name, ok := columnNameMap.LoadValue(ecolumn.Key)
				if !ok {
					panic(fmt.Sprintf("missing name mapping for %d", ecolumn.Key))
				}
				column.ClickHouseTransformFrom[idx].Name = name
			}
		}

		// Non-main columns with an alias are NotSortingKey
		if !column.ClickHouseMainOnly && column.ClickHouseAlias != "" {
			column.ClickHouseNotSortingKey = true
		}

		// Transform implicit dependencies
		for idx := range column.ClickHouseTransformFrom {
			deps := column.ClickHouseTransformFrom[idx].Depends
			deps = append(deps, column.Key)
			slices.Sort(deps)
			column.ClickHouseTransformFrom[idx].Depends = slices.Compact(deps)
			column.Depends = append(column.Depends, column.ClickHouseTransformFrom[idx].Key)
		}
		slices.Sort(column.Depends)
		column.Depends = slices.Compact(column.Depends)

		ncolumns = append(ncolumns, column)

		// Expand the schema Src → Dst and InIf → OutIf
		alreadyExists := func(name string) bool {
			key, _ := columnNameMap.LoadKey(name)
			for _, column := range schema.columns {
				if column.Key == key {
					return true
				}
			}
			return false
		}
		if strings.HasPrefix(column.Name, "Src") {
			column.Name = fmt.Sprintf("Dst%s", column.Name[3:])
			if !alreadyExists(column.Name) {
				column.Key, ok = columnNameMap.LoadKey(column.Name)
				if !ok {
					panic(fmt.Sprintf("missing name mapping for %q", column.Name))
				}
				column.ClickHouseAlias = strings.ReplaceAll(column.ClickHouseAlias, "Src", "Dst")
				column.ClickHouseTransformFrom = slices.Clone(column.ClickHouseTransformFrom)
				ncolumns = append(ncolumns, column)
			}
		} else if strings.HasPrefix(column.Name, "InIf") {
			column.Name = fmt.Sprintf("OutIf%s", column.Name[4:])
			if !alreadyExists(column.Name) {
				column.Key, ok = columnNameMap.LoadKey(column.Name)
				if !ok {
					panic(fmt.Sprintf("missing name mapping for %q", column.Name))
				}
				column.ClickHouseAlias = strings.ReplaceAll(column.ClickHouseAlias, "InIf", "OutIf")
				column.ClickHouseTransformFrom = slices.Clone(column.ClickHouseTransformFrom)
				ncolumns = append(ncolumns, column)
			}
		}
	}
	schema.columns = ncolumns

	// Set Protobuf index and type
	protobufIndex := 1
	ncolumns = []Column{}
	for _, column := range schema.columns {
		pcolumns := []*Column{&column}
		for idx := range column.ClickHouseTransformFrom {
			pcolumns = append(pcolumns, &column.ClickHouseTransformFrom[idx])
		}
		for _, column := range pcolumns {
			if column.ProtobufIndex == 0 {
				if column.ClickHouseTransformFrom != nil ||
					column.ClickHouseGenerateFrom != "" ||
					column.ClickHouseAlias != "" {
					column.ProtobufIndex = -1
					continue
				}

				column.ProtobufIndex = protowire.Number(protobufIndex)
				protobufIndex++
			}

			if column.ProtobufType == 0 &&
				column.ClickHouseTransformFrom == nil &&
				column.ClickHouseGenerateFrom == "" &&
				column.ClickHouseAlias == "" {
				switch column.ClickHouseType {
				case "String", "LowCardinality(String)", "FixedString(2)":
					column.ProtobufType = protoreflect.StringKind
				case "UInt64":
					column.ProtobufType = protoreflect.Uint64Kind
				case "UInt32", "UInt16", "UInt8":
					column.ProtobufType = protoreflect.Uint32Kind
				case "IPv6", "LowCardinality(IPv6)":
					column.ProtobufType = protoreflect.BytesKind
				case "Array(UInt32)":
					column.ProtobufType = protoreflect.Uint32Kind
					column.ProtobufRepeated = true
				}
			}
		}
		ncolumns = append(ncolumns, column)
	}
	schema.columns = ncolumns

	// Build column index
	schema.columnIndex = make([]*Column, ColumnLast+schema.dynamicColumns)
	for i, column := range schema.columns {
		schema.columnIndex[column.Key] = &schema.columns[i]
		for j, column := range column.ClickHouseTransformFrom {
			schema.columnIndex[column.Key] = &schema.columns[i].ClickHouseTransformFrom[j]
		}
	}

	// Update disabledGroups
	schema.disabledGroups = *bitset.New(uint(ColumnGroupLast))
	for group := ColumnGroup(0); group < ColumnGroupLast; group++ {
		schema.disabledGroups.Set(uint(group))
		for _, column := range schema.columns {
			if !column.Disabled && column.Group == group {
				schema.disabledGroups.Clear(uint(group))
			}
		}
	}

	return schema
}
