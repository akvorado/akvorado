// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "testing"

func TestCapitalize(t *testing.T) {
	cases := []struct {
		In  string
		Out string
	}{
		{"", ""},
		{"Hello", "Hello"},
		{"bye", "Bye"},
		{" nothing", " nothing"},
		{"école", "École"},
	}
	for _, tc := range cases {
		got := Capitalize(tc.In)
		if diff := Diff(got, tc.Out); diff != "" {
			t.Errorf("Capitalize(%q) (-got, +want):\n%s", tc.In, diff)
		}
	}
}
