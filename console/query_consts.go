// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import "akvorado/common/helpers"

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
	queryColumnSrcNetPrefix
	queryColumnDstAS
	queryColumnDstASPath
	queryColumnDst1stAS
	queryColumnDst2ndAS
	queryColumnDst3rdAS
	queryColumnDstCommunities
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
	queryColumnDstNetPrefix
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
	queryColumnSrcNetPrefix:      "SrcNetPrefix",
	queryColumnDstNetPrefix:      "DstNetPrefix",
	queryColumnSrcAS:             "SrcAS",
	queryColumnDstAS:             "DstAS",
	queryColumnDstASPath:         "DstASPath",
	queryColumnDst1stAS:          "Dst1stAS",
	queryColumnDst2ndAS:          "Dst2ndAS",
	queryColumnDst3rdAS:          "Dst3rdAS",
	queryColumnDstCommunities:    "DstCommunities",
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
