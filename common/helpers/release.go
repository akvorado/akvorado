// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build release

package helpers

// Testing reports whether the current code is being run in a test. It always
// return false in release mode and therefore its test has no performance
// impact.
func Testing() bool {
	return false
}
