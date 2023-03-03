// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"akvorado/common/schema"
	"akvorado/console/query"
)

func TestRequireMainTable(t *testing.T) {
	cases := []struct {
		Columns  []query.Column
		Filter   query.Filter
		Expected bool
	}{
		{[]query.Column{}, query.NewFilter(""), false},
		{[]query.Column{query.NewColumn("SrcAS")}, query.NewFilter(""), false},
		{[]query.Column{query.NewColumn("ExporterAddress")}, query.NewFilter(""), false},
		{[]query.Column{query.NewColumn("SrcPort")}, query.NewFilter(""), true},
		{[]query.Column{query.NewColumn("SrcAddr")}, query.NewFilter(""), true},
		{[]query.Column{query.NewColumn("DstPort")}, query.NewFilter(""), true},
		{[]query.Column{query.NewColumn("DstAddr")}, query.NewFilter(""), true},
		{[]query.Column{query.NewColumn("SrcAS"), query.NewColumn("DstAddr")}, query.NewFilter(""), true},
		{[]query.Column{query.NewColumn("DstAddr"), query.NewColumn("SrcAS")}, query.NewFilter(""), true},
		{[]query.Column{query.NewColumn("DstNetPrefix")}, query.NewFilter(""), true},
		{[]query.Column{}, query.NewFilter("SrcAddr = 203.0.113.15"), true},
	}
	sch := schema.NewMock(t)
	for idx, tc := range cases {
		if err := query.Columns(tc.Columns).Validate(sch); err != nil {
			t.Fatalf("Validate() error:\n%+v", err)
		}
		if err := tc.Filter.Validate(sch); err != nil {
			t.Fatalf("Validate() error:\n%+v", err)
		}
		got := requireMainTable(sch, tc.Columns, tc.Filter)
		if got != tc.Expected {
			t.Errorf("requireMainTable(%d) == %v but expected %v", idx, got, tc.Expected)
		}
	}
}
