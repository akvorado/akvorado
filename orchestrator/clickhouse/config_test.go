// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"

	"github.com/gin-gonic/gin"

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
			Configuration: func() any { return gin.H{} },
			Expected:      &helpers.SubnetMap[NetworkAttributes]{},
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv4",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return gin.H{"203.0.113.0/24": gin.H{"name": "customer"}} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"::ffff:203.0.113.0/120": {Name: "customer"},
			}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv6",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return gin.H{"2001:db8:1::/64": gin.H{"name": "customer"}} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"2001:db8:1::/64": {Name: "customer"},
			}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv4 subnet (compatibility)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return gin.H{"203.0.113.0/24": "customer"} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"::ffff:203.0.113.0/120": {Name: "customer"},
			}),
		}, {
			Pos:           helpers.Mark(),
			Description:   "IPv6 subnet (compatibility)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return gin.H{"2001:db8:1::/64": "customer"} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"2001:db8:1::/64": {Name: "customer"},
			}),
		}, {
			Pos:         helpers.Mark(),
			Description: "all attributes",
			Initial:     func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any {
				return gin.H{"203.0.113.0/24": gin.H{
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
			Configuration: func() any { return gin.H{"192.0.2.1/38": "customer"} },
			Error:         true,
		}, {
			Pos:           helpers.Mark(),
			Description:   "Invalid subnet (2)",
			Initial:       func() any { return &helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() any { return gin.H{"192.0.2.1/255.0.255.0": "customer"} },
			Error:         true,
		},
	})
}

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}

func init() {
	helpers.RegisterSubnetMapCmp[NetworkAttributes]()
}
