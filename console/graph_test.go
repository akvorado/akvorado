// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

func TestSourceSelect(t *testing.T) {
	sch := schema.NewMock(t)
	cases := []struct {
		Description string
		Input       graphCommonHandlerInput
		Expected    string
	}{
		{
			Description: "no dimensions",
			Input: graphCommonHandlerInput{
				Dimensions: []query.Column{},
			},
			Expected: "SELECT * FROM flows_1m0s SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "no truncatable dimensions",
			Input: graphCommonHandlerInput{
				Dimensions:     []query.Column{query.NewColumn("ExporterAddress")},
				TruncateAddrV4: 16,
				TruncateAddrV6: 40,
			},
			Expected: "SELECT * FROM flows_1m0s SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "no truncatation",
			Input: graphCommonHandlerInput{
				Dimensions: []query.Column{query.NewColumn("SrcAddr")},
			},
			Expected: "SELECT * FROM flows_1m0s SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "IPv4/IPv6 same prefix length",
			Input: graphCommonHandlerInput{
				Dimensions:     []query.Column{query.NewColumn("SrcAddr")},
				TruncateAddrV4: 16,
				TruncateAddrV6: 112,
			},
			Expected: "SELECT * REPLACE (tupleElement(IPv6CIDRToRange(SrcAddr, 112), 1) AS SrcAddr) FROM flows_1m0s SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "IPv4/IPv6 different prefix length",
			Input: graphCommonHandlerInput{
				Dimensions:     []query.Column{query.NewColumn("SrcAddr")},
				TruncateAddrV4: 24,
				TruncateAddrV6: 40,
			},
			Expected: "SELECT * REPLACE (tupleElement(IPv6CIDRToRange(SrcAddr, if(tupleElement(IPv6CIDRToRange(SrcAddr, 96), 1) = toIPv6('0.0.0.0'), 120, 40)), 1) AS SrcAddr) FROM flows_1m0s SETTINGS asterisk_include_alias_columns = 1",
		},
	}
	for _, tc := range cases {
		tc.Input.schema = sch
		if err := query.Columns(tc.Input.Dimensions).Validate(tc.Input.schema); err != nil {
			t.Fatalf("Validate() error:\n%+v", err)
		}
		got := sb.Normalize(t, tc.Input.sourceSelect("flows_1m0s").String())
		if diff := helpers.Diff(got, sb.Normalize(t, tc.Expected)); diff != "" {
			t.Errorf("sourceSelect(%q) (-got, +want): \n%s", tc.Description, diff)
		}
	}
}

func TestGraphReverseHandler(t *testing.T) {
	_, h, _, _ := NewMock(t, DefaultConfiguration())
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "dimensions and filter",
			URL:         "/api/v0/console/graph/reverse",
			JSONInput: helpers.M{
				"dimensions": []string{"SrcAS", "InIfProvider", "ExporterName"},
				"filter":     `InIfBoundary = external AND srcas = AS65000`,
			},
			JSONOutput: helpers.M{
				"dimensions": []string{"DstAS", "OutIfProvider", "ExporterName"},
				"filter":     `OutIfBoundary = external AND DstAS = AS65000`,
			},
		}, {
			Description: "empty",
			URL:         "/api/v0/console/graph/reverse",
			JSONInput:   helpers.M{"filter": ""},
			JSONOutput: helpers.M{
				"dimensions": []string{},
				"filter":     "",
			},
		}, {
			Description: "unknown dimension",
			URL:         "/api/v0/console/graph/reverse",
			JSONInput: helpers.M{
				"dimensions": []string{"Nothing"},
				"filter":     "",
			},
			StatusCode: 400,
			JSONOutput: helpers.M{"message": "Unknown column name Nothing"},
		}, {
			Description: "invalid filter",
			URL:         "/api/v0/console/graph/reverse",
			JSONInput: helpers.M{
				"dimensions": []string{"SrcAS"},
				"filter":     `InIfName = "`,
			},
			StatusCode: 400,
			JSONOutput: helpers.M{"message": "at line 1, position 12: string literal not terminated"},
		},
	})
}
