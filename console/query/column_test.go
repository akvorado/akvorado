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
			t.Fatalf("UnmarshalText(%q) error:\n%+v", tc.Input, err)
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
	sch := schema.NewMock(t)
	cases := []struct {
		Input    schema.ColumnKey
		Expected string
	}{
		{
			Input:    schema.ColumnSrcAddr,
			Expected: `replaceRegexpOne(IPv6NumToString(SrcAddr), '^::ffff:', '')`,
		}, {
			Input:    schema.ColumnDstAddrNAT,
			Expected: `replaceRegexpOne(IPv6NumToString(DstAddrNAT), '^::ffff:', '')`,
		}, {
			Input:    schema.ColumnExporterAddress,
			Expected: `replaceRegexpOne(IPv6NumToString(ExporterAddress), '^::ffff:', '')`,
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
			Input:    schema.ColumnDstVlan,
			Expected: `toString(DstVlan)`,
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
		}, {
			Input:    schema.ColumnInIfBoundary,
			Expected: `toString(InIfBoundary)`,
		}, {
			Input:    schema.ColumnMPLSLabels,
			Expected: `arrayStringConcat(MPLSLabels, ' ')`,
		}, {
			Input:    schema.ColumnMPLS3rdLabel,
			Expected: `toString(MPLS3rdLabel)`,
		}, {
			Input: schema.ColumnTCPFlags,
			// Can be tested with "WITH 16 AS TCPFlags SELECT ..."
			Expected: `arrayStringConcat([if(bitTest(TCPFlags, 0) = 1, 'F', ''), if(bitTest(TCPFlags, 1) = 1, 'S', ''), if(bitTest(TCPFlags, 2) = 1, 'R', ''), if(bitTest(TCPFlags, 3) = 1, 'P', ''), if(bitTest(TCPFlags, 4) = 1, '.', ''), if(bitTest(TCPFlags, 5) = 1, 'U', ''), if(bitTest(TCPFlags, 6) = 1, 'E', ''), if(bitTest(TCPFlags, 7) = 1, 'C', ''), if(bitTest(TCPFlags, 8) = 1, 'N', '')], '')`,
		}, {
			Input:    schema.ColumnDstPort,
			Expected: "replaceRegexpOne(multiIf(Proto==6, concat(toString(DstPort), '/', dictGetOrDefault('tcp', 'name', DstPort,'')), Proto==17, concat(toString(DstPort), '/', dictGetOrDefault('udp', 'name', DstPort,'')), toString(DstPort)), '/$', '')",
		}, {
			Input:    schema.ColumnSrcPort,
			Expected: "replaceRegexpOne(multiIf(Proto==6, concat(toString(SrcPort), '/', dictGetOrDefault('tcp', 'name', SrcPort,'')), Proto==17, concat(toString(SrcPort), '/', dictGetOrDefault('udp', 'name', SrcPort,'')), toString(SrcPort)), '/$', '')",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Input.String(), func(t *testing.T) {
			column := query.NewColumn(tc.Input.String())
			if err := column.Validate(schema.NewMock(t).EnableAllColumns()); err != nil {
				t.Fatalf("Validate() error:\n%+v", err)
			}
			got := column.ToSQLSelect(sch)
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
