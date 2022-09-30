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
		{[]queryColumn{queryColumnSrcAS}, queryFilter{}, false},
		{[]queryColumn{queryColumnExporterAddress}, queryFilter{}, false},
		{[]queryColumn{queryColumnSrcPort}, queryFilter{}, true},
		{[]queryColumn{queryColumnSrcAddr}, queryFilter{}, true},
		{[]queryColumn{queryColumnDstPort}, queryFilter{}, true},
		{[]queryColumn{queryColumnDstAddr}, queryFilter{}, true},
		{[]queryColumn{queryColumnSrcAS, queryColumnDstAddr}, queryFilter{}, true},
		{[]queryColumn{queryColumnDstAddr, queryColumnSrcAS}, queryFilter{}, true},
		{[]queryColumn{}, queryFilter{MainTableRequired: true}, true},
	}
	for idx, tc := range cases {
		got := requireMainTable(tc.Columns, tc.Filter)
		if got != tc.Expected {
			t.Errorf("requireMainTable(%d) == %v but expected %v", idx, got, tc.Expected)
		}
	}
}

func TestQueryColumnSQLSelect(t *testing.T) {
	cases := []struct {
		Input    queryColumn
		Expected string
	}{
		{
			Input:    queryColumnSrcAddr,
			Expected: `replaceRegexpOne(IPv6NumToString(SrcAddr), '^::ffff:', '')`,
		}, {
			Input:    queryColumnDstAS,
			Expected: `concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???'))`,
		}, {
			Input:    queryColumnDst2ndAS,
			Expected: `concat(toString(Dst2ndAS), ': ', dictGetOrDefault('asns', 'name', Dst2ndAS, '???'))`,
		}, {
			Input:    queryColumnProto,
			Expected: `dictGetOrDefault('protocols', 'name', Proto, '???')`,
		}, {
			Input:    queryColumnEType,
			Expected: `if(EType = 2048, 'IPv4', if(EType = 34525, 'IPv6', '???'))`,
		}, {
			Input:    queryColumnOutIfSpeed,
			Expected: `toString(OutIfSpeed)`,
		}, {
			Input:    queryColumnExporterName,
			Expected: `ExporterName`,
		}, {
			Input:    queryColumnPacketSizeBucket,
			Expected: `PacketSizeBucket`,
		}, {
			Input:    queryColumnDstASPath,
			Expected: `arrayStringConcat(DstASPath, ' ')`,
		}, {
			Input:    queryColumnDstCommunities,
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
