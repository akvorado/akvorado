// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import "testing"

func TestValidateVersion(t *testing.T) {
	ok := []string{
		"24.1.1111.1111",
		"23.6.1111.1111",
		"23.5.3.24",
		"23.3.8.21",
		"23.3.3.52",
		"23.2.7.32",
		"22.12.5.34",
		"22.8.16.32",
		"22.4.3.3",
	}
	for _, v := range ok {
		if err := validateVersion(v); err != nil {
			t.Errorf("validateVersion(%q) error:\n%+v", v, err)
		}
	}
	nok := []string{
		"23.4.1.1943",
		"23.3.1.2823",
		"23.2.2.20",
		"23.1.1.3077",
		"22.3.11.12",
	}
	for _, v := range nok {
		if err := validateVersion(v); err == nil {
			t.Errorf("validateVersion(%q) did not error", v)
		}
	}
}
