// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

type versionRange struct {
	min string
	max string
}

// validateVersionRanges validate that ClickHouse version is not between the
// provided ranges.
func validateVersionRanges(current *version.Version, reason string, invalidRanges []versionRange) error {
	for _, versions := range invalidRanges {
		v1 := version.Must(version.NewVersion(versions.min))
		v2 := version.Must(version.NewVersion(versions.max))
		if current.GreaterThanOrEqual(v1) && current.LessThan(v2) {
			return fmt.Errorf("incompatible ClickHouse version %s (%s): upgrade to %s",
				current, reason, v2)
		}
	}
	return nil
}

// validateVersion checks if ClickHouse version is supported.
func validateVersion(v string) error {
	current, err := version.NewVersion(v)
	if err != nil {
		return fmt.Errorf("cannot parse version %q", v)
	}

	{
		// Check minimum supported version
		minVersion := version.Must(version.NewVersion("22.4"))
		if !current.GreaterThanOrEqual(minVersion) {
			return fmt.Errorf("required minimal version of ClickHouse is 22.4 (got %s)", current)
		}
	}

	{
		// Check for IPv6 encoding problems in 23.x
		// See: https://github.com/ClickHouse/ClickHouse/issues/49924
		nok := []versionRange{
			{"23", "23.2.7.32"},
			{"23.2", "23.2.7.23"},
			{"23.3", "23.3.3.52"},
			{"23.4", "23.4.3.48"},
			{"23.5", "23.5.1.3174"},
		}
		if err := validateVersionRanges(current, "IPv6 protobuf encoding bug", nok); err != nil {
			return err
		}
	}
	{
		// Check for experimental analyzer and INTERPOLATE issue
		// Introduced by: https://github.com/ClickHouse/ClickHouse/pull/61652
		// Fixed in: https://github.com/ClickHouse/ClickHouse/pull/64096
		nok := []versionRange{
			{"24.3", "24.3.4.147"},
			{"24.4", "24.4.2.141"},
			{"24.5", "24.5.1.1763"},
		}
		if err := validateVersionRanges(current, "experimental analyzer bug with INTERPOLATE", nok); err != nil {
			return err
		}
	}

	return nil
}
