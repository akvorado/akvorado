// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package query_test

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/query"
)

func TestUnmarshalFilter(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
		Error    bool
	}{
		{"", "", false},
		{"   ", "", false},
		{"SrcPort=12322", "SrcPort = 12322", false},
		{"NoPort = 12322", "", true},
	}
	sch := schema.NewMock(t)
	for _, tc := range cases {
		t.Run(tc.Input, func(t *testing.T) {
			var qf query.Filter
			if err := qf.UnmarshalText([]byte(tc.Input)); err != nil {
				t.Fatalf("UnmarshalText() error:\n%+v", err)
			}
			err := qf.Validate(sch, "")
			if err != nil && !tc.Error {
				t.Fatalf("Validate() error:\n%+v", err)
			}
			if err == nil && tc.Error {
				t.Fatal("Validate() did not error")
			}
			if err != nil {
				return
			}
			if diff := helpers.Diff(qf.Direct().String(), tc.Expected); diff != "" {
				t.Fatalf("UnmarshalText(%q) (-got, +want):\n%s", tc.Input, diff)
			}
		})
	}
}

func TestFilterMainTableRequired(t *testing.T) {
	// DstPort is moved out of the main table, while SrcPort stays in it. A
	// filter on either one needs the main table, because a filter can be
	// swapped to get the reverse direction.
	sch, err := schema.New(schema.Configuration{
		NotMainTableOnly: []schema.ColumnKey{schema.ColumnDstPort},
	})
	if err != nil {
		t.Fatalf("schema.New() error:\n%+v", err)
	}
	cases := []struct {
		Input    string
		Expected bool
	}{
		{"SrcPort = 80", true},
		{"DstPort = 80", true},
		{"SrcAS = 12322", false},
	}
	for _, tc := range cases {
		t.Run(tc.Input, func(t *testing.T) {
			qf := query.NewFilter(tc.Input)
			if err := qf.Validate(sch, ""); err != nil {
				t.Fatalf("Validate() error:\n%+v", err)
			}
			if diff := helpers.Diff(qf.MainTableRequired(), tc.Expected); diff != "" {
				t.Errorf("MainTableRequired(%q) (-got, +want):\n%s", tc.Input, diff)
			}
		})
	}
}

func TestFilterSwap(t *testing.T) {
	filter := query.NewFilter("SrcAS = 12322")
	if err := filter.Validate(schema.NewMock(t), ""); err != nil {
		t.Fatalf("Validate() error:\n%+v", err)
	}
	filter.Swap()
	if diff := helpers.Diff(filter.Direct().String(), "DstAS = 12322"); diff != "" {
		t.Fatalf("Swap() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(filter.Reverse().String(), "SrcAS = 12322"); diff != "" {
		t.Fatalf("Swap() (-got, +want):\n%s", diff)
	}
}
