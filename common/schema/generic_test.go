// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import "testing"

func TestLookupColumnByName(t *testing.T) {
	cases := []string{
		"TimeReceived",
		"InIfProvider",
		"OutIfProvider",
		"SrcAS",
		"ForwardingStatus",
	}
	for _, name := range cases {
		column, ok := Flows.LookupColumnByName(name)
		if !ok {
			t.Fatalf("LookupByName(%q) not found", name)
		}
		if column.Name != name {
			t.Fatalf("LookupByName(%q) == %q but should be %q", name, column.Name, name)
		}
	}
}

func TestReverseColumnDirection(t *testing.T) {
	cases := []struct {
		Input  ColumnKey
		Output ColumnKey
	}{
		{ColumnSrcAS, ColumnDstAS},
		{ColumnDstAS, ColumnSrcAS},
		{ColumnInIfProvider, ColumnOutIfProvider},
		{ColumnOutIfDescription, ColumnInIfDescription},
		{ColumnDstASPath, ColumnDstASPath},
		{ColumnExporterName, ColumnExporterName},
	}
	for _, tc := range cases {
		got := Flows.ReverseColumnDirection(tc.Input)
		if got != tc.Output {
			t.Errorf("ReverseColumnDirection(%q) == %q but expected %q",
				tc.Input.String(), got.String(), tc.Output.String())
		}
	}
}
