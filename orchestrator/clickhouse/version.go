// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"fmt"

	"github.com/hashicorp/go-version"
)

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
		nok := []struct {
			min string
			max string
		}{
			{"23", "23.2.7.32"},
			{"23.2", "23.2.7.23"},
			{"23.3", "23.3.3.52"},
			{"23.4", "23.4.3.48"},
			{"23.5", "23.5.1.3174"},
		}
		for _, versions := range nok {
			v1 := version.Must(version.NewVersion(versions.min))
			v2 := version.Must(version.NewVersion(versions.max))
			if current.GreaterThanOrEqual(v1) && current.LessThan(v2) {
				return fmt.Errorf("incompatible ClickHouse version %s (IPv6 protobuf encoding bug): upgrade to %s",
					current, v2)
			}
		}
	}

	return nil
}
