// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/openconfig/gnmic/pkg/api"
)

// gnmiOptions returns the list of GNMIOptions to poll for a given model.
func (m Model) gnmiOptions(options ...api.GNMIOption) []api.GNMIOption {
	appendPaths := func(paths []string) {
		for _, path := range paths {
			options = append(options, api.Subscription(api.Path(path)))
		}
	}
	appendPaths(m.SystemNamePaths)
	appendPaths(m.IfIndexPaths)
	appendPaths(m.IfNamePaths)
	appendPaths(m.IfDescriptionPaths)
	for _, path := range m.IfSpeedPaths {
		options = append(options, api.Subscription(api.Path(path.Path)))
	}
	return options
}

// convertSpeed converts a speed to an integer value in Mbps.
func convertSpeed(strValue string, unit IfSpeedPathUnit) (uint, error) {
	switch unit {
	case SpeedBits:
		val, err := strconv.ParseUint(strValue, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint(val / 1_000_000), nil
	case SpeedMegabits:
		val, err := strconv.ParseUint(strValue, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint(val), nil
	case SpeedHuman:
		return convertHumanSpeed(strValue)
	case SpeedEthernet:
		if strValue == "SPEED_UNKNOWN" {
			return 0, nil
		}
		if !strings.HasPrefix(strValue, "SPEED_") || !strings.HasSuffix(strValue, "B") {
			return 0, fmt.Errorf("unknown speed %s", strValue)
		}
		strValue = strValue[6 : len(strValue)-1]
		return convertHumanSpeed(strValue)
	default:
		panic(fmt.Errorf("unknown speed format %d", unit))
	}
}

// convertHumanSpeed converts a speed expressed as 10G or 100M to a value in Mbps.
func convertHumanSpeed(strValue string) (uint, error) {
	l := len(strValue)
	multiplier := strValue[l-1]
	strValue = strValue[:l-1]
	val, err := strconv.ParseUint(strValue, 10, 32)
	if err != nil {
		return 0, err
	}
	switch multiplier {
	case 'K':
		return uint(val) / 1000, nil
	case 'M':
		return uint(val), nil
	case 'G':
		return uint(val) * 1000, nil
	case 'T':
		return uint(val) * 1_000_000, nil
	default:
		return 0, fmt.Errorf("unknown multiplier %c", multiplier)
	}
}
