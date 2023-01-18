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
			err := qf.Validate(sch)
			if err != nil && !tc.Error {
				t.Fatalf("Validate() error:\n%+v", err)
			}
			if err == nil && tc.Error {
				t.Fatal("Validate() did not error")
			}
			if err != nil {
				return
			}
			if diff := helpers.Diff(qf.Direct(), tc.Expected); diff != "" {
				t.Fatalf("UnmarshalText(%q) (-got, +want):\n%s", tc.Input, diff)
			}
		})
	}
}

func TestFilterSwap(t *testing.T) {
	filter := query.NewFilter("SrcAS = 12322")
	if err := filter.Validate(schema.NewMock(t)); err != nil {
		t.Fatalf("Validate() error:\n%+v", err)
	}
	filter.Swap()
	if diff := helpers.Diff(filter.Direct(), "DstAS = 12322"); diff != "" {
		t.Fatalf("Swap() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(filter.Reverse(), "SrcAS = 12322"); diff != "" {
		t.Fatalf("Swap() (-got, +want):\n%s", diff)
	}
}
