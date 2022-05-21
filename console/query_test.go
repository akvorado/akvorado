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
			Expected: `if(EType = 0x800, 'IPv4', if(EType = 0x86dd, 'IPv6', '???'))`,
		}, {
			Input:    queryColumnOutIfSpeed,
			Expected: `toString(OutIfSpeed)`,
		}, {
			Input:    queryColumnExporterName,
			Expected: `ExporterName`,
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
