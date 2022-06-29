// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"testing"

	"akvorado/common/helpers"
)

func TestListenValidator(t *testing.T) {
	s := struct {
		Listen string `validate:"listen"`
	}{}
	cases := []struct {
		Listen string
		Err    bool
	}{
		{"127.0.0.1:161", false},
		{"localhost:161", false},
		{"0.0.0.0:161", false},
		{"0.0.0.0:0", false},
		{"127.0.0.1:0", false},
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.1:what", true},
		{"127.0.0.1:100000", true},
	}
	for _, tc := range cases {
		s.Listen = tc.Listen
		err := helpers.Validate.Struct(s)
		if err == nil && tc.Err {
			t.Error("Validate.Struct() expected an error")
		} else if err != nil && !tc.Err {
			t.Errorf("Validate.Struct() error:\n%+v", err)
		}
	}
}
