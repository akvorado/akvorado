// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"akvorado/common/helpers"
)

func TestQueryColumnSQLSelect(t *testing.T) {
	cases := []struct {
		Input    queryColumn
		Expected string
	}{
		{
			Input:    queryColumnSrcAddr,
			Expected: `IPv6NumToString(SrcAddr)`,
		}, {
			Input:    queryColumnDstAS,
			Expected: `concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???'))`,
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
			if diff := helpers.Diff(qf.filter, tc.Expected); diff != "" {
				t.Fatalf("UnmarshalText(%q) (-got, +want):\n%s", tc.Input, diff)
			}
		})
	}
}
