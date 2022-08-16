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
			Description:   "nil",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return nil },
			Expected:      helpers.SubnetMap[NetworkAttributes]{},
		}, {
			Description:   "empty",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{} },
			Expected:      helpers.SubnetMap[NetworkAttributes]{},
		}, {
			Description:   "IPv4",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{"203.0.113.0/24": gin.H{"name": "customer"}} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"::ffff:203.0.113.0/120": {Name: "customer"},
			}),
		}, {
			Description:   "IPv6",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{"2001:db8:1::/64": gin.H{"name": "customer"}} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"2001:db8:1::/64": {Name: "customer"},
			}),
		}, {
			Description:   "IPv4 subnet (compatibility)",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{"203.0.113.0/24": "customer"} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"::ffff:203.0.113.0/120": {Name: "customer"},
			}),
		}, {
			Description:   "IPv6 subnet (compatibility)",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{"2001:db8:1::/64": "customer"} },
			Expected: helpers.MustNewSubnetMap(map[string]NetworkAttributes{
				"2001:db8:1::/64": {Name: "customer"},
			}),
		}, {
			Description: "all attributes",
			Initial:     func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} {
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
			Description:   "Invalid subnet (1)",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{"192.0.2.1/38": "customer"} },
			Error:         true,
		}, {
			Description:   "Invalid subnet (2)",
			Initial:       func() interface{} { return helpers.SubnetMap[NetworkAttributes]{} },
			Configuration: func() interface{} { return gin.H{"192.0.2.1/255.0.255.0": "customer"} },
			Error:         true,
		},
	})
}

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	config.Kafka.Topic = "flow"
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
