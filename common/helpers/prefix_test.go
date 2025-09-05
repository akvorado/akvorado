// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"net/netip"
	"testing"

	"akvorado/common/helpers"
)

func TestUnmapPrefix(t *testing.T) {
	for _, tc := range []struct {
		input  string
		output string
	}{
		{"0.0.0.0/0", "0.0.0.0/0"},
		{"::/0", "::/0"},
		{"192.168.12.0/24", "192.168.12.0/24"},
		{"2001:db8::/52", "2001:db8::/52"},
		{"::ffff:192.168.12.0/120", "192.168.12.0/24"},
		{"::ffff:0.0.0.0/0", "::ffff:0.0.0.0/0"},
		{"::ffff:0.0.0.0/96", "0.0.0.0/0"},
	} {
		prefix := netip.MustParsePrefix(tc.input)
		got := helpers.UnmapPrefix(prefix).String()
		if diff := helpers.Diff(got, tc.output); diff != "" {
			t.Errorf("UnmapPrefix(%q) (-got, +want):\n%s", tc.input, diff)
		}
	}
}
