// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "unicode"

// Capitalize turns the first letter of a string to its upper case version.
func Capitalize(str string) string {
	if len(str) == 0 {
		return ""
	}
	r := []rune(str)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
