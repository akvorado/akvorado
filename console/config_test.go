// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestConfigHandler(t *testing.T) {
	config := DefaultConfiguration()
	_, h, _, _ := NewMock(t, config)
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/configuration",
			JSONOutput: gin.H{
				"version": "dev",
				"defaultVisualizeOptions": gin.H{
					"graphType":      "stacked",
					"start":          "6 hours ago",
					"end":            "now",
					"filter":         "InIfBoundary = external",
					"dimensions":     []string{"SrcAS"},
					"limit":          10,
					"limitType":      "avg",
					"bidirectional":  false,
					"previousPeriod": false,
				},
				"homepageTopWidgets": []string{"src-as", "src-port", "protocol", "src-country", "etype"},
				"dimensionsLimit":    50,
				"dimensions": []string{
					"ExporterAddress",
					"ExporterName",
					"ExporterGroup",
					"ExporterRole",
					"ExporterSite",
					"ExporterRegion",
					"ExporterTenant",
					"SrcAddr",
					"DstAddr",
					"SrcNetPrefix",
					"DstNetPrefix",
					"SrcAS",
					"DstAS",
					"SrcNetName",
					"DstNetName",
					"SrcNetRole",
					"DstNetRole",
					"SrcNetSite",
					"DstNetSite",
					"SrcNetRegion",
					"DstNetRegion",
					"SrcNetTenant",
					"DstNetTenant",
					"SrcCountry",
					"DstCountry",
					"SrcGeoCity",
					"DstGeoCity",
					"SrcGeoState",
					"DstGeoState",
					"DstASPath",
					"Dst1stAS",
					"Dst2ndAS",
					"Dst3rdAS",
					"DstCommunities",
					"InIfName",
					"OutIfName",
					"InIfDescription",
					"OutIfDescription",
					"InIfSpeed",
					"OutIfSpeed",
					"InIfConnectivity",
					"OutIfConnectivity",
					"InIfProvider",
					"OutIfProvider",
					"InIfBoundary",
					"OutIfBoundary",
					"EType",
					"Proto",
					"SrcPort",
					"DstPort",
					"PacketSizeBucket",
					"ForwardingStatus",
				},
				"truncatable": []string{"SrcAddr", "DstAddr"},
			},
		},
	})
}
