// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import (
	"encoding/binary"
	"iter"
	"testing"
)

// Generate all n-permutations of the provided array. Duplicates may exist. The array will be mutated.
func permutations[T any](orig []T, n int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		if n > len(orig) {
			return
		}
		// Special case for 0
		if n == 0 {
			yield([]T{})
			return
		}

		for p := make([]int, len(orig)); p[0] < len(p); {
			result := append([]T{}, orig...)
			for i, v := range p {
				result[i], result[i+v] = result[i+v], result[i]
			}
			yield(result[:n])
			for i := len(p) - 1; i >= 0; i-- {
				if i == 0 || p[i] < len(p)-i-1 {
					p[i]++
					return
				}
				p[i] = 0
			}
		}
	}
}
func TestDecodeUNumber(t *testing.T) {
	bytes := []byte{0x31, 0x1, 0xa8, 0x11, 0xc1, 0x1, 0xd9, 0x11}
	for l := range len(bytes) {
		for p := range permutations(bytes, l) {
			got := decodeUNumber(p)
			bytes8 := append(make([]byte, 8-l), p...)
			expected := binary.BigEndian.Uint64(bytes8)
			if got != expected {
				t.Fatalf("decodeUNumber(%v) = %d, expected %d", p, got, expected)
			}
		}
	}
}
