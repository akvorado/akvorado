// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package query_test

import (
	"fmt"
	"reflect"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/query"
)

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
		var qc query.Column
		if err := qc.UnmarshalText([]byte(tc.Input)); err != nil {
			t.Fatalf("UnmarshalText() error:\n%+v", err)
		}
		err := qc.Validate(schema.NewMock(t))
		if err != nil && !tc.Error {
			t.Fatalf("Validate(%q) error:\n%+v", tc.Input, err)
		}
		if err == nil && tc.Error {
			t.Fatalf("Validate(%q) did not error", tc.Input)
		}
		if err != nil {
			continue
		}
		if diff := helpers.Diff(qc.Key(), tc.Expected, helpers.DiffFormatter(reflect.TypeOf(schema.ColumnBytes), fmt.Sprint)); diff != "" {
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
		}, {
			Input:    schema.ColumnDstMAC,
			Expected: `MACNumToString(DstMAC)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Input.String(), func(t *testing.T) {
			column := query.NewColumn(tc.Input.String())
			if err := column.Validate(schema.NewMock(t).EnableAllColumns()); err != nil {
				t.Fatalf("Validate() error:\n%+v", err)
			}
			got := column.ToSQLSelect()
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("toSQLWhere (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestReverseDirection(t *testing.T) {
	columns := query.Columns{
		query.NewColumn("SrcAS"),
		query.NewColumn("DstAS"),
		query.NewColumn("ExporterName"),
		query.NewColumn("InIfProvider"),
	}
	sch := schema.NewMock(t)
	if err := columns.Validate(sch); err != nil {
		t.Fatalf("Validate() error:\n%+v", err)
	}
	columns.Reverse(sch)
	expected := query.Columns{
		query.NewColumn("DstAS"),
		query.NewColumn("SrcAS"),
		query.NewColumn("ExporterName"),
		query.NewColumn("OutIfProvider"),
	}
	if diff := helpers.Diff(columns, expected, helpers.DiffFormatter(reflect.TypeOf(query.Column{}), fmt.Sprint)); diff != "" {
		t.Fatalf("Reverse() (-got, +want):\n%s", diff)
	}
}
