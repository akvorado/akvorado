// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"testing"

	"akvorado/common/helpers"
)

func TestConvertSpeed(t *testing.T) {
	cases := []struct {
		Value    string
		Unit     IfSpeedPathUnit
		Expected uint
		Error    bool
	}{
		{
			Value:    "1000",
			Unit:     SpeedBits,
			Expected: 0,
		}, {
			Value:    "1000000",
			Unit:     SpeedBits,
			Expected: 1,
		}, {
			Value:    "100000000",
			Unit:     SpeedBits,
			Expected: 100,
		}, {
			Value: "-43874",
			Unit:  SpeedBits,
			Error: true,
		}, {
			Value: "nope",
			Unit:  SpeedBits,
			Error: true,
		}, {
			Value:    "1",
			Unit:     SpeedMegabits,
			Expected: 1,
		}, {
			Value: "-1",
			Unit:  SpeedMegabits,
			Error: true,
		}, {
			Value:    "2500",
			Unit:     SpeedMegabits,
			Expected: 2500,
		}, {
			Value:    "10G",
			Unit:     SpeedHuman,
			Expected: 10000,
		}, {
			Value:    "25G",
			Unit:     SpeedHuman,
			Expected: 25000,
		}, {
			Value:    "100M",
			Unit:     SpeedHuman,
			Expected: 100,
		}, {
			Value:    "100000K",
			Unit:     SpeedHuman,
			Expected: 100,
		}, {
			Value: "100000S",
			Unit:  SpeedHuman,
			Error: true,
		}, {
			Value: "&G",
			Unit:  SpeedHuman,
			Error: true,
		}, {
			Value:    "1T",
			Unit:     SpeedHuman,
			Expected: 1000000,
		}, {
			Value:    "SPEED_100GB",
			Unit:     SpeedEthernet,
			Expected: 100000,
		}, {
			Value:    "SPEED_25GB",
			Unit:     SpeedEthernet,
			Expected: 25000,
		}, {
			Value:    "SPEED_10MB",
			Unit:     SpeedEthernet,
			Expected: 10,
		}, {
			Value: "SPEED_10M",
			Unit:  SpeedEthernet,
			Error: true,
		}, {
			Value: "10MB",
			Unit:  SpeedEthernet,
			Error: true,
		}, {
			Value:    "SPEED_UNKNOWN",
			Unit:     SpeedEthernet,
			Expected: 0,
		}, {
			Value: "SPEED_XXGB",
			Unit:  SpeedEthernet,
			Error: true,
		},
	}
	for _, tc := range cases {
		got, err := convertSpeed(tc.Value, tc.Unit)
		if err != nil && !tc.Error {
			t.Errorf("convertSpeed(%q, %s) error:\n%+v", tc.Value, tc.Unit, err)
		} else if err == nil && tc.Error {
			t.Errorf("convertSpeed(%q, %s) no error", tc.Value, tc.Unit)
		} else if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("convertSpeed(%q, %s) (-got, +want):\n%s", tc.Value, tc.Unit, diff)
		}
	}

}
