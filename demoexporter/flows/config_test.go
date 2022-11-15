// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"net/netip"
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	config.Flows = []FlowConfiguration{
		{
			PerSecond:             10,
			InIfIndex:             []int{1},
			OutIfIndex:            []int{2},
			PeakHour:              21 * time.Hour,
			Multiplier:            3.0,
			SrcNet:                netip.MustParsePrefix("2001:db8:1::/64"),
			DstNet:                netip.MustParsePrefix("2001:db8:2::/64"),
			SrcAS:                 []uint32{2906},
			DstAS:                 []uint32{12322},
			SrcPort:               []uint16{443},
			Protocol:              []string{"tcp"},
			Size:                  1400,
			ReverseDirectionRatio: 0.2,
		},
	}
	config.Target = "127.0.0.1:2055"
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
