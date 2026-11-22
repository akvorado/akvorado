// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

// IsPrivateAS tells if an AS number is reserved or private.
func IsPrivateAS(as uint32) bool {
	// See https://www.iana.org/assignments/iana-as-numbers-special-registry/iana-as-numbers-special-registry.xhtml
	if as == 0 || as == 23456 {
		return true
	}
	if 64496 <= as && as <= 65551 || 4_200_000_000 <= as {
		return true
	}
	return false
}
