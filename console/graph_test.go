// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
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
			Expected: "SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "no truncatable dimensions",
			Input: graphCommonHandlerInput{
				Dimensions:     []query.Column{query.NewColumn("ExporterAddress")},
				TruncateAddrV4: 16,
				TruncateAddrV6: 40,
			},
			Expected: "SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "no truncatation",
			Input: graphCommonHandlerInput{
				Dimensions: []query.Column{query.NewColumn("SrcAddr")},
			},
			Expected: "SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "IPv4/IPv6 same prefix length",
			Input: graphCommonHandlerInput{
				Dimensions:     []query.Column{query.NewColumn("SrcAddr")},
				TruncateAddrV4: 16,
				TruncateAddrV6: 112,
			},
			Expected: "SELECT * REPLACE (tupleElement(IPv6CIDRToRange(SrcAddr, 112), 1) AS SrcAddr) FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1",
		}, {
			Description: "IPv4/IPv6 different prefix length",
			Input: graphCommonHandlerInput{
				Dimensions:     []query.Column{query.NewColumn("SrcAddr")},
				TruncateAddrV4: 24,
				TruncateAddrV6: 40,
			},
			Expected: "SELECT * REPLACE (tupleElement(IPv6CIDRToRange(SrcAddr, if(tupleElement(IPv6CIDRToRange(SrcAddr, 96), 1) = toIPv6('::ffff:0.0.0.0'), 120, 40)), 1) AS SrcAddr) FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1",
		},
	}
	for _, tc := range cases {
		tc.Input.schema = sch
		if err := query.Columns(tc.Input.Dimensions).Validate(tc.Input.schema); err != nil {
			t.Fatalf("Validate() error:\n%+v", err)
		}
		got := tc.Input.sourceSelect()
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("sourceSelect(%q) (-got, +want): \n%s", tc.Description, diff)
		}
	}
}
