// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

func TestRequireMainTable(t *testing.T) {
	cases := []struct {
		Columns  []queryColumn
		Filter   queryFilter
		Expected bool
	}{
		{[]queryColumn{}, queryFilter{}, false},
		{[]queryColumn{queryColumn(schema.ColumnSrcAS)}, queryFilter{}, false},
		{[]queryColumn{queryColumn(schema.ColumnExporterAddress)}, queryFilter{}, false},
		{[]queryColumn{queryColumn(schema.ColumnSrcPort)}, queryFilter{}, true},
		{[]queryColumn{queryColumn(schema.ColumnSrcAddr)}, queryFilter{}, true},
		{[]queryColumn{queryColumn(schema.ColumnDstPort)}, queryFilter{}, true},
		{[]queryColumn{queryColumn(schema.ColumnDstAddr)}, queryFilter{}, true},
		{[]queryColumn{queryColumn(schema.ColumnSrcAS), queryColumn(schema.ColumnDstAddr)}, queryFilter{}, true},
		{[]queryColumn{queryColumn(schema.ColumnDstAddr), queryColumn(schema.ColumnSrcAS)}, queryFilter{}, true},
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
		Expected schema.ColumnKey
		Error    bool
	}{
		{"DstAddr", schema.ColumnDstAddr, false},
		{"TimeReceived", 0, true},
		{"Nothing", 0, true},
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
		Input    schema.ColumnKey
		Expected string
	}{
		{
			Input:    schema.ColumnSrcAddr,
			Expected: `replaceRegexpOne(IPv6NumToString(SrcAddr), '^::ffff:', '')`,
		}, {
			Input:    schema.ColumnDstAS,
			Expected: `concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???'))`,
		}, {
			Input:    schema.ColumnDst2ndAS,
			Expected: `concat(toString(Dst2ndAS), ': ', dictGetOrDefault('asns', 'name', Dst2ndAS, '???'))`,
		}, {
			Input:    schema.ColumnProto,
			Expected: `dictGetOrDefault('protocols', 'name', Proto, '???')`,
		}, {
			Input:    schema.ColumnEType,
			Expected: `if(EType = 2048, 'IPv4', if(EType = 34525, 'IPv6', '???'))`,
		}, {
			Input:    schema.ColumnOutIfSpeed,
			Expected: `toString(OutIfSpeed)`,
		}, {
			Input:    schema.ColumnExporterName,
			Expected: `ExporterName`,
		}, {
			Input:    schema.ColumnPacketSizeBucket,
			Expected: `PacketSizeBucket`,
		}, {
			Input:    schema.ColumnDstASPath,
			Expected: `arrayStringConcat(DstASPath, ' ')`,
		}, {
			Input:    schema.ColumnDstCommunities,
			Expected: `arrayStringConcat(arrayConcat(arrayMap(c -> concat(toString(bitShiftRight(c, 16)), ':', toString(bitAnd(c, 0xffff))), DstCommunities), arrayMap(c -> concat(toString(bitAnd(bitShiftRight(c, 64), 0xffffffff)), ':', toString(bitAnd(bitShiftRight(c, 32), 0xffffffff)), ':', toString(bitAnd(c, 0xffffffff))), DstLargeCommunities)), ' ')`,
		},
	}
	for _, tc := range cases {
		t.Run(queryColumn(tc.Input).String(), func(t *testing.T) {
			got := queryColumn(tc.Input).toSQLSelect()
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
