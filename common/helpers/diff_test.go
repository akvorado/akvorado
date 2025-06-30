// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "testing"

func TestDiffStringBytes(t *testing.T) {
	type TestStruct struct {
		A any
		B any
	}
	got := TestStruct{
		A: "hello",
		B: []byte("bye"),
	}
	want := TestStruct{
		A: "hello",
		B: "bye",
	}
	if diff := Diff(got, want); diff == "" {
		// We expect a diff if we have []byte in one case and string in another.
		// The test is mostly for self-documentation of this behavior.
		t.Fatalf("Diff() (-got, +want):\n%s", diff)
	}
}
