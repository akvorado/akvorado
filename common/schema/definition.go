// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"errors"
	"fmt"
	"strings"

	"akvorado/common/helpers/bimap"

	"github.com/bits-and-blooms/bitset"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// InterfaceBoundary identifies wether the interface is facing inside or outside the network.
type InterfaceBoundary uint

const (
	// InterfaceBoundaryUndefined means we don't know about the interface.
	InterfaceBoundaryUndefined InterfaceBoundary = iota
	// InterfaceBoundaryExternal means this interface is facing outside our network
	InterfaceBoundaryExternal
	// InterfaceBoundaryInternal means this interface is facing inside our network
	InterfaceBoundaryInternal
)

var (
	interfaceBoundaryMap = bimap.New(map[InterfaceBoundary]string{
		InterfaceBoundaryUndefined: "undefined",
		InterfaceBoundaryExternal:  "external",
		InterfaceBoundaryInternal:  "internal",
	})
	errUnknownInterfaceBoundary = errors.New("unknown interface boundary")
)

// MarshalText turns an interface boundary to text
func (ib InterfaceBoundary) MarshalText() ([]byte, error) {
	got, ok := interfaceBoundaryMap.LoadValue(ib)
	if ok {
		return []byte(got), nil
	}
	return nil, errUnknownInterfaceBoundary
}

// String turns an interface boundary to string
func (ib InterfaceBoundary) String() string {
	got, _ := interfaceBoundaryMap.LoadValue(ib)
	return got
}

// UnmarshalText provides an interface boundary from text
func (ib *InterfaceBoundary) UnmarshalText(input []byte) error {
	if len(input) == 0 {
		*ib = InterfaceBoundaryUndefined
		return nil
	}
	got, ok := interfaceBoundaryMap.LoadKey(string(input))
	if ok {
		*ib = got
		return nil
	}
	return errUnknownInterfaceBoundary
}

const (
	// DictionaryASNs is the name of the asns clickhouse dictionary.
	DictionaryASNs string = "asns"
	// DictionaryProtocols is the name of the protocols clickhouse dictionary.
	DictionaryProtocols string = "protocols"
	// DictionaryICMP is the name of the icmp clickhouse dictionary.
	DictionaryICMP string = "icmp"
	// DictionaryNetworks is the name of the networks clickhouse dictionary.
	DictionaryNetworks string = "networks"
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
	ColumnSrcGeoState
	ColumnDstGeoState
	ColumnSrcGeoCity
	ColumnDstGeoCity
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
	ColumnMPLSLabels
	ColumnMPLS1stLabel
	ColumnMPLS2ndLabel
	ColumnMPLS3rdLabel
	ColumnMPLS4thLabel

	// ColumnLast points to after the last static column, custom dictionaries
	// (dynamic columns) come after ColumnLast
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
			{Key: ColumnExporterAddress, ParserType: "ip", ClickHouseType: "LowCardinality(IPv6)"},
			{Key: ColumnExporterName, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterGroup, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterRole, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterSite, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterRegion, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnExporterTenant, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{
				Key:                ColumnSrcAddr,
				ParserType:         "ip",
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
				Key:                        ColumnSrcNetPrefix,
				ClickHouseMainOnly:         true,
				ClickHouseType:             "String",
				ClickHouseMaterializedType: "LowCardinality(String)",
				ClickHouseAlias: `CASE
 WHEN EType = 0x800 THEN concat(replaceRegexpOne(IPv6CIDRToRange(SrcAddr, (96 + SrcNetMask)::UInt8).1::String, '^::ffff:', ''), '/', SrcNetMask::String)
 WHEN EType = 0x86dd THEN concat(IPv6CIDRToRange(SrcAddr, SrcNetMask).1::String, '/', SrcNetMask::String)
 ELSE ''
END`,
			},
			{
				Key:                     ColumnSrcAS,
				ClickHouseType:          "UInt32",
				ClickHouseGenerateFrom:  "if(SrcAS = 0, c_SrcNetworks[asn], SrcAS)",
				ClickHouseSelfGenerated: true,
			},
			{
				Key:                     ColumnDstAS,
				ClickHouseType:          "UInt32",
				ClickHouseGenerateFrom:  "if(DstAS = 0, c_DstNetworks[asn], DstAS)",
				ClickHouseSelfGenerated: true,
			},
			{
				Key:                    ColumnSrcNetName,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[name]",
			},
			{
				Key:                    ColumnDstNetName,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[name]",
			},
			{
				Key:                    ColumnSrcNetRole,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[role]",
			},
			{
				Key:                    ColumnDstNetRole,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[role]",
			},
			{
				Key:                    ColumnSrcNetSite,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[site]",
			},
			{
				Key:                    ColumnDstNetSite,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[site]",
			},
			{
				Key:                    ColumnSrcNetRegion,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[region]",
			},
			{
				Key:                    ColumnDstNetRegion,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[region]",
			},
			{
				Key:                    ColumnSrcNetTenant,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[tenant]",
			},
			{
				Key:                    ColumnDstNetTenant,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[tenant]",
			},
			{Key: ColumnSrcVlan, ParserType: "uint", ClickHouseType: "UInt16", Disabled: true, Group: ColumnGroupL2},
			{
				Key:                    ColumnSrcCountry,
				ParserType:             "string",
				ClickHouseType:         "FixedString(2)",
				ClickHouseGenerateFrom: "c_SrcNetworks[country]",
			},
			{
				Key:                    ColumnDstCountry,
				ParserType:             "string",
				ClickHouseType:         "FixedString(2)",
				ClickHouseGenerateFrom: "c_DstNetworks[country]",
			},
			{
				Key:                    ColumnSrcGeoCity,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[city]",
			},
			{
				Key:                    ColumnDstGeoCity,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[city]",
			},
			{
				Key:                    ColumnSrcGeoState,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_SrcNetworks[state]",
			},
			{
				Key:                    ColumnDstGeoState,
				ParserType:             "string",
				ClickHouseType:         "LowCardinality(String)",
				ClickHouseGenerateFrom: "c_DstNetworks[state]",
			},
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
			{Key: ColumnInIfName, ParserType: "string", ClickHouseType: "LowCardinality(String)"},
			{Key: ColumnInIfDescription, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnInIfSpeed, ParserType: "uint", ClickHouseType: "UInt32", ClickHouseNotSortingKey: true},
			{Key: ColumnInIfConnectivity, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{Key: ColumnInIfProvider, ParserType: "string", ClickHouseType: "LowCardinality(String)", ClickHouseNotSortingKey: true},
			{
				Key:                     ColumnInIfBoundary,
				ClickHouseType:          fmt.Sprintf("Enum8('undefined' = %d, 'external' = %d, 'internal' = %d)", InterfaceBoundaryUndefined, InterfaceBoundaryExternal, InterfaceBoundaryInternal),
				ClickHouseNotSortingKey: true,
				ProtobufType:            protoreflect.EnumKind,
				ProtobufEnumName:        "Boundary",
				ProtobufEnum: map[int]string{
					int(InterfaceBoundaryUndefined): "UNDEFINED",
					int(InterfaceBoundaryExternal):  "EXTERNAL",
					int(InterfaceBoundaryInternal):  "INTERNAL",
				},
			},
			{Key: ColumnEType, ClickHouseType: "UInt32"}, // TODO: UInt16 but hard to change, primary key
			{Key: ColumnProto, ClickHouseType: "UInt32"}, // TODO: UInt8 but hard to change, primary key
			{Key: ColumnSrcPort, ParserType: "uint", ClickHouseType: "UInt16", ClickHouseMainOnly: true},
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
				ParserType:          "uint",
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
			{Key: ColumnForwardingStatus, ParserType: "uint", ClickHouseType: "UInt32"}, // TODO: UInt8 but hard to change, primary key
			{
				Key:                ColumnSrcAddrNAT,
				Disabled:           true,
				Group:              ColumnGroupNAT,
				ParserType:         "ip",
				ClickHouseType:     "IPv6",
				ClickHouseMainOnly: true,
				ConsoleTruncateIP:  true,
			},
			{
				Key:                ColumnSrcPortNAT,
				Disabled:           true,
				Group:              ColumnGroupNAT,
				ParserType:         "uint",
				ClickHouseType:     "UInt16",
				ClickHouseMainOnly: true,
			},
			{Key: ColumnSrcMAC, Disabled: true, Group: ColumnGroupL2, ClickHouseType: "UInt64"},
			{Key: ColumnIPTTL, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt8"},
			{Key: ColumnIPTos, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt8"},
			{Key: ColumnIPFragmentID, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt32"},
			{Key: ColumnIPFragmentOffset, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt16"},
			{Key: ColumnIPv6FlowLabel, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt32"},
			{Key: ColumnTCPFlags, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt16"},
			{Key: ColumnICMPv4Type, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt8"},
			{Key: ColumnICMPv4Code, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt8"},
			{Key: ColumnICMPv6Type, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt8"},
			{Key: ColumnICMPv6Code, Disabled: true, Group: ColumnGroupL3L4, ParserType: "uint", ClickHouseType: "UInt8"},
			{
				Key:            ColumnICMPv4,
				Depends:        []ColumnKey{ColumnProto, ColumnICMPv4Type, ColumnICMPv4Code},
				Disabled:       true,
				Group:          ColumnGroupL3L4,
				ParserType:     "string",
				ClickHouseType: "LowCardinality(String)",
				ClickHouseAlias: `if(Proto = 1, ` +
					fmt.Sprintf(`dictGetOrDefault('%s', 'name', tuple(Proto, ICMPv4Type, ICMPv4Code), `, DictionaryICMP) +
					`concat(toString(ICMPv4Type), '/', toString(ICMPv4Code))), '')`,
			},
			{
				Key:            ColumnICMPv6,
				Depends:        []ColumnKey{ColumnProto, ColumnICMPv6Type, ColumnICMPv6Code},
				Disabled:       true,
				Group:          ColumnGroupL3L4,
				ParserType:     "string",
				ClickHouseType: "LowCardinality(String)",
				ClickHouseAlias: `if(Proto = 58, ` +
					fmt.Sprintf(`dictGetOrDefault('%s', 'name', tuple(Proto, ICMPv6Type, ICMPv6Code), `, DictionaryICMP) +
					`concat(toString(ICMPv6Type), '/', toString(ICMPv6Code))), '')`,
			},
			{
				Key:             ColumnNextHop,
				Disabled:        true,
				ParserType:      "ip",
				ClickHouseType:  "LowCardinality(IPv6)",
				ClickHouseCodec: "ZSTD(1)",
			},
			{
				Key:                ColumnMPLSLabels,
				Disabled:           true,
				ClickHouseMainOnly: true,
				ClickHouseType:     "Array(UInt32)",
				ParserType:         "array(uint)",
			},
			{
				Key:                ColumnMPLS1stLabel,
				Disabled:           true,
				Depends:            []ColumnKey{ColumnMPLSLabels},
				ClickHouseMainOnly: true,
				ClickHouseType:     "UInt32",
				ClickHouseAlias:    "MPLSLabels[1]",
				ParserType:         "uint",
			},
			{
				Key:                ColumnMPLS2ndLabel,
				Disabled:           true,
				Depends:            []ColumnKey{ColumnMPLSLabels},
				ClickHouseMainOnly: true,
				ClickHouseType:     "UInt32",
				ClickHouseAlias:    "MPLSLabels[2]",
				ParserType:         "uint",
			},
			{
				Key:                ColumnMPLS3rdLabel,
				Disabled:           true,
				Depends:            []ColumnKey{ColumnMPLSLabels},
				ClickHouseMainOnly: true,
				ClickHouseType:     "UInt32",
				ClickHouseAlias:    "MPLSLabels[3]",
				ParserType:         "uint",
			},
			{
				Key:                ColumnMPLS4thLabel,
				Disabled:           true,
				Depends:            []ColumnKey{ColumnMPLSLabels},
				ClickHouseMainOnly: true,
				ClickHouseType:     "UInt32",
				ClickHouseAlias:    "MPLSLabels[4]",
				ParserType:         "uint",
			},
		},
	}.finalize()
}

func (column *Column) shouldBeProto() bool {
	return column.ClickHouseTransformFrom == nil &&
		(column.ClickHouseGenerateFrom == "" || column.ClickHouseSelfGenerated) &&
		column.ClickHouseAlias == ""
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
				if !column.shouldBeProto() {
					column.ProtobufIndex = -1
					continue
				}

				column.ProtobufIndex = protowire.Number(protobufIndex)
				protobufIndex++
			}

			if column.ProtobufType == 0 &&
				column.shouldBeProto() {
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
	maxKey := ColumnTimeReceived
	for _, column := range schema.columns {
		if column.Key > maxKey {
			maxKey = column.Key
		}
	}
	schema.columnIndex = make([]*Column, maxKey+1)
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
