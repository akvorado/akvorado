// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestTableSettingsDecode(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Pos:         helpers.Mark(),
			Description: "string and int settings",
			Initial:     func() any { return &ResolutionConfiguration{} },
			Configuration: func() any {
				return helpers.M{
					"interval": "5m",
					"ttl":      "360h",
					"table-settings": helpers.M{
						"storage_policy":         "ssd",
						"merge_with_ttl_timeout": 3600,
					},
				}
			},
			Expected: &ResolutionConfiguration{
				Interval: 5 * time.Minute,
				TTL:      360 * time.Hour,
				TableSettings: TableSettings{
					"storage_policy":         "ssd",
					"merge_with_ttl_timeout": 3600,
				},
			},
		},
		{
			Pos:         helpers.Mark(),
			Description: "invalid key with special characters",
			Initial:     func() any { return &ResolutionConfiguration{} },
			Configuration: func() any {
				return helpers.M{
					"interval": "5m",
					"ttl":      "360h",
					"table-settings": helpers.M{
						"storage policy": "ssd",
					},
				}
			},
			Error: true,
		},
		{
			Pos:         helpers.Mark(),
			Description: "invalid key with SQL injection",
			Initial:     func() any { return &ResolutionConfiguration{} },
			Configuration: func() any {
				return helpers.M{
					"interval": "5m",
					"ttl":      "360h",
					"table-settings": helpers.M{
						"'; DROP TABLE flows --": "ssd",
					},
				}
			},
			Error: true,
		},
	})
}

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
