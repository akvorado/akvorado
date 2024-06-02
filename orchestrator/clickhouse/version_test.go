// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import "testing"

func TestValidateVersionOK(t *testing.T) {
	ok := []string{
		// IPv6 encoding issue
		"24.1.1111.1111",
		"23.6.1111.1111",
		"23.5.3.24",
		"23.3.8.21",
		"23.3.3.52",
		"23.2.7.32",
		"22.12.5.34",
		"22.8.16.32",
		"22.4.3.3",

		// Experimental analyzer and INTERPOLATE
		"24.5.1.1763", // fixed version
		"24.2.3.70",   // no experimental analyzer
	}
	for _, v := range ok {
		if err := validateVersion(v); err != nil {
			t.Errorf("validateVersion(%q) error:\n%+v", v, err)
		}
	}
}

func TestValidateVersionNOK(t *testing.T) {
	nok := []string{
		// Too old
		"22.3.11.12",

		// IPv6 encoding issue
		"23.4.1.1943",
		"23.3.1.2823",
		"23.2.2.20",
		"23.1.1.3077",

		// Experimental analyzer and INTERPOLATE
		"24.4.1.2088", // not fixed yet
		"24.3.3.102",  // not fixed yet
	}
	for _, v := range nok {
		if err := validateVersion(v); err == nil {
			t.Errorf("validateVersion(%q) did not error", v)
		}
	}
}
