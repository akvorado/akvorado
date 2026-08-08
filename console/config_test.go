// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"github.com/benbjohnson/clock"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
)

// TestHomepageGraphFilter checks the filter is parsed when the console starts.
// An invalid one has to be reported then, not on each request to the homepage.
func TestHomepageGraphFilter(t *testing.T) {
	cases := []struct {
		Description string
		Filter      string
		Invalid     bool
	}{
		{Description: "no filter"},
		{Description: "a condition", Filter: "InIfBoundary = 'external'"},
		{Description: "incomplete condition", Filter: "InIfBoundary =", Invalid: true},
		{Description: "two conditions", Filter: "InIfBoundary = 'external', SrcAS = 65000", Invalid: true},
		{
			// Anything after the expression lands in a clause of the SELECT it
			// is parsed in, where it would be dropped without notice.
			Description: "condition followed by a clause",
			Filter:      "InIfBoundary = 'external' LIMIT 1",
			Invalid:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			r := reporter.NewMock(t)
			ch, _ := clickhousedb.NewMock(t, r)
			config := DefaultConfiguration()
			config.HomepageGraphFilter = tc.Filter
			_, err := New(r, config, Dependencies{
				Daemon:       daemon.NewMock(t),
				HTTP:         httpserver.NewMock(t, r),
				ClickHouseDB: ch,
				Clock:        clock.NewMock(),
				Auth:         authentication.NewMock(t, r),
				Database:     database.NewMock(t, r, database.DefaultConfiguration()),
				Schema:       schema.NewMock(t),
			})
			if tc.Invalid && err == nil {
				t.Errorf("New() with filter %q did not return an error", tc.Filter)
			}
			if !tc.Invalid && err != nil {
				t.Errorf("New() error:\n%+v", err)
			}
		})
	}
}

func TestConfigHandler(t *testing.T) {
	config := DefaultConfiguration()
	_, h, _, _ := NewMock(t, config)
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/configuration",
			JSONOutput: helpers.M{
				"version": "dev",
				"defaultVisualizeOptions": helpers.M{
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
					"PacketSize",
					"PacketSizeBucket",
					"ForwardingStatus",
					"FlowDirection",
				},
				"truncatable": []string{"SrcAddr", "DstAddr"},
				"branding":    false,
			},
		},
	})
}
