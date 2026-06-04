// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package pb

import "testing"

func TestDecapsulationProtocolPBBText(t *testing.T) {
	for _, input := range []string{"pbb", "spbm", "mac-in-mac"} {
		var got RawFlow_DecapsulationProtocol
		if err := got.UnmarshalText([]byte(input)); err != nil {
			t.Fatalf("UnmarshalText(%q) error:\n%+v", input, err)
		}
		if got != RawFlow_DECAP_PBB {
			t.Fatalf("UnmarshalText(%q) = %v, expected %v", input, got, RawFlow_DECAP_PBB)
		}
	}

	got, err := RawFlow_DECAP_PBB.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error:\n%+v", err)
	}
	if string(got) != "pbb" {
		t.Fatalf("MarshalText() = %q, expected pbb", got)
	}
}
