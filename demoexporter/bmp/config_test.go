// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"testing"

	"akvorado/common/helpers"
)

func TestCommunity(t *testing.T) {
	cases := []struct {
		Input    string
		Expected Community
		Error    bool
	}{
		{"12322:10", 807534602, false},
		{"0:100", 100, false},
		{"1:0", 65536, false},
		{"65536:1", 0, true},
		{"12322:65536", 0, true},
		{"kfjgkf", 0, true},
		{"fdgj:gffg", 0, true},
	}
	for _, tc := range cases {
		var got Community
		err := got.UnmarshalText([]byte(tc.Input))
		if err == nil && tc.Error {
			t.Errorf("UnmarshalText(%q) did not error", tc.Input)
		} else if err != nil && !tc.Error {
			t.Errorf("UnmarshalText(%q) error:\n%+v", tc.Input, err)
		} else if err == nil && got != tc.Expected {
			t.Errorf("UnmarshalText(%q) == %d, expected %d", tc.Input, got, tc.Expected)
		} else if err == nil && got.String() != tc.Input {
			t.Errorf("%q.String() == %s", tc.Input, got.String())
		}
	}
}

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
