// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestNetworkNamesUnmarshalHook(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Pos:           helpers.Mark(),
			Description:   "nil",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return nil },
			Expected:      &helpers.SubnetMap[NetworkAttributes]{},
		}, {
			Pos:           helpers.Mark(),
			Description:   "empty",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{} },
			Expected:      &helpers.SubnetMap[NetworkAttributes]{},
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv4",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{"203.0.113.0/24": helpers.M{"name": "customer"}} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"::ffff:203.0.113.0/120": {Name: "customer"},
			}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv6",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{"2001:db8:1::/64": helpers.M{"name": "customer"}} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"2001:db8:1::/64": {Name: "customer"},
			}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv4 subnet (compatibility)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{"203.0.113.0/24": "customer"} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"::ffff:203.0.113.0/120": {Name: "customer"},
			}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv6 subnet (compatibility)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{"2001:db8:1::/64": "customer"} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"2001:db8:1::/64": {Name: "customer"},
			}),
		}, {
			Pos:         helpers.Mark(),
			Description: "all attributes",
			Initial:     func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any {
				return helpers.M{"203.0.113.0/24": helpers.M{
					"name":   "customer1",
					"role":   "customer",
					"site":   "paris",
					"region": "france",
					"tenant": "mobile",
				}}
			},
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{"::ffff:203.0.113.0/120": {
				Name:   "customer1",
				Role:   "customer",
				Site:   "paris",
				Region: "france",
				Tenant: "mobile",
			}}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "Invalid subnet (1)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{"192.0.2.1/38": "customer"} },
			Error:         true,
		}, {
			Pos:           helpers.Mark(),
			Description:   "Invalid subnet (2)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return helpers.M{"192.0.2.1/255.0.255.0": "customer"} },
			Error:         true,
		},
	})
}

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

func TestBloomFPPValidation(t *testing.T) {
	for _, tc := range []struct {
		fpp   float64
		valid bool
	}{
		{0.001, true},
		{0.5, true},
		{0.999, true},
		{0, true}, // is seen as default, gets set to 0.001 in migrations
		{1, false},
		{1.5, false},
		{-0.1, false},
	} {
		config := DefaultConfiguration()
		config.BloomFPP = tc.fpp
		err := helpers.Validate.Struct(config)
		if tc.valid && err != nil {
			t.Errorf("BloomFPP=%v: expected valid, got error: %v", tc.fpp, err)
		} else if !tc.valid && err == nil {
			t.Errorf("BloomFPP=%v: expected invalid, got no error", tc.fpp)
		}
	}
}

func init() {
	helpers.RegisterSubnetMapCmp[NetworkAttributes]()
}
