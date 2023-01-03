// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"akvorado/common/helpers"
)

func TestRequireMainTable(t *testing.T) {
	cases := []struct {
		Columns  []queryColumn
		Filter   queryFilter
		Expected bool
	}{
		{[]queryColumn{}, queryFilter{}, false},
		{[]queryColumn{"SrcAS"}, queryFilter{}, false},
		{[]queryColumn{"ExporterAddress"}, queryFilter{}, false},
		{[]queryColumn{"SrcPort"}, queryFilter{}, true},
		{[]queryColumn{"SrcAddr"}, queryFilter{}, true},
		{[]queryColumn{"DstPort"}, queryFilter{}, true},
		{[]queryColumn{"DstAddr"}, queryFilter{}, true},
		{[]queryColumn{"SrcAS", "DstAddr"}, queryFilter{}, true},
		{[]queryColumn{"DstAddr", "SrcAS"}, queryFilter{}, true},
		{[]queryColumn{}, queryFilter{MainTableRequired: true}, true},
	}
	for idx, tc := range cases {
		got := requireMainTable(tc.Columns, tc.Filter)
		if got != tc.Expected {
			t.Errorf("requireMainTable(%d) == %v but expected %v", idx, got, tc.Expected)
		}
	}
}

func TestUnmarshalQueryColumn(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
		Error    bool
	}{
		{"DstAddr", "DstAddr", false},
		{"TimeReceived", "", true},
		{"Nothing", "", true},
	}
	for _, tc := range cases {
		var qc queryColumn
		err := qc.UnmarshalText([]byte(tc.Input))
		if err != nil && !tc.Error {
			t.Fatalf("UnmarshalText(%q) error:\n%+v", tc.Input, err)
		}
		if err == nil && tc.Error {
			t.Fatalf("UnmarshalText(%q) did not error", tc.Input)
		}
		if diff := helpers.Diff(qc, tc.Expected); diff != "" {
			t.Fatalf("UnmarshalText(%q) (-got, +want):\n%s", tc.Input, diff)
		}
	}
}

func TestQueryColumnSQLSelect(t *testing.T) {
	cases := []struct {
		Input    queryColumn
		Expected string
	}{
		{
			Input:    "SrcAddr",
			Expected: `replaceRegexpOne(IPv6NumToString(SrcAddr), '^::ffff:', '')`,
		}, {
			Input:    "DstAS",
			Expected: `concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???'))`,
		}, {
			Input:    "Dst2ndAS",
			Expected: `concat(toString(Dst2ndAS), ': ', dictGetOrDefault('asns', 'name', Dst2ndAS, '???'))`,
		}, {
			Input:    "Proto",
			Expected: `dictGetOrDefault('protocols', 'name', Proto, '???')`,
		}, {
			Input:    "EType",
			Expected: `if(EType = 2048, 'IPv4', if(EType = 34525, 'IPv6', '???'))`,
		}, {
			Input:    "OutIfSpeed",
			Expected: `toString(OutIfSpeed)`,
		}, {
			Input:    "ExporterName",
			Expected: `ExporterName`,
		}, {
			Input:    "PacketSizeBucket",
			Expected: `PacketSizeBucket`,
		}, {
			Input:    "DstASPath",
			Expected: `arrayStringConcat(DstASPath, ' ')`,
		}, {
			Input:    "DstCommunities",
			Expected: `arrayStringConcat(arrayConcat(arrayMap(c -> concat(toString(bitShiftRight(c, 16)), ':', toString(bitAnd(c, 0xffff))), DstCommunities), arrayMap(c -> concat(toString(bitAnd(bitShiftRight(c, 64), 0xffffffff)), ':', toString(bitAnd(bitShiftRight(c, 32), 0xffffffff)), ':', toString(bitAnd(c, 0xffffffff))), DstLargeCommunities)), ' ')`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Input.String(), func(t *testing.T) {
			got := tc.Input.toSQLSelect()
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("toSQLWhere (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestUnmarshalFilter(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
	}{
		{"", ""},
		{"   ", ""},
		{"SrcPort=12322", "SrcPort = 12322"},
	}
	for _, tc := range cases {
		t.Run(tc.Input, func(t *testing.T) {
			var qf queryFilter
			err := qf.UnmarshalText([]byte(tc.Input))
			if err != nil {
				t.Fatalf("UnmarshalText(%q) error:\n%+v", tc.Input, err)
			}
			if diff := helpers.Diff(qf.Filter, tc.Expected); diff != "" {
				t.Fatalf("UnmarshalText(%q) (-got, +want):\n%s", tc.Input, diff)
			}
		})
	}
}
