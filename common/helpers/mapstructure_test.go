// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "testing"

func TestMapStructureMatchName(t *testing.T) {
	cases := []struct {
		mapKey    string
		fieldName string
		expected  bool
	}{
		{"one", "one", true},
		{"one", "One", true},
		{"one-two", "OneTwo", true},
		{"onetwo", "OneTwo", true},
		{"One-Two", "OneTwo", true},
		{"two", "one", false},
	}
	for _, tc := range cases {
		got := MapStructureMatchName(tc.mapKey, tc.fieldName)
		if got && !tc.expected {
			t.Errorf("%q == %q but expected !=", tc.mapKey, tc.fieldName)
		} else if !got && tc.expected {
			t.Errorf("%q != %q but expected ==", tc.mapKey, tc.fieldName)
		}
	}
}
