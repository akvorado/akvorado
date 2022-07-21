// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"akvorado/common/helpers"
)

func TestNetworkNamesUnmarshalHook(t *testing.T) {
	cases := []struct {
		Description string
		Input       map[string]interface{}
		Output      NetworkMap
	}{
		{
			Description: "nil",
			Input:       nil,
			Output:      NetworkMap{},
		}, {
			Description: "empty",
			Input:       gin.H{},
			Output:      NetworkMap{},
		}, {
			Description: "IPv4",
			Input:       gin.H{"203.0.113.0/24": gin.H{"name": "customer"}},
			Output:      NetworkMap{"::ffff:203.0.113.0/120": NetworkAttributes{Name: "customer"}},
		}, {
			Description: "IPv6",
			Input:       gin.H{"2001:db8:1::/64": gin.H{"name": "customer"}},
			Output:      NetworkMap{"2001:db8:1::/64": NetworkAttributes{Name: "customer"}},
		}, {
			Description: "IPv4 subnet (compatibility)",
			Input:       gin.H{"203.0.113.0/24": "customer"},
			Output:      NetworkMap{"::ffff:203.0.113.0/120": NetworkAttributes{Name: "customer"}},
		}, {
			Description: "IPv6 subnet (compatibility)",
			Input:       gin.H{"2001:db8:1::/64": "customer"},
			Output:      NetworkMap{"2001:db8:1::/64": NetworkAttributes{Name: "customer"}},
		}, {
			Description: "all attributes",
			Input: gin.H{"203.0.113.0/24": gin.H{
				"name":   "customer1",
				"role":   "customer",
				"site":   "paris",
				"region": "france",
				"tenant": "mobile",
			}},
			Output: NetworkMap{"::ffff:203.0.113.0/120": NetworkAttributes{
				Name:   "customer1",
				Role:   "customer",
				Site:   "paris",
				Region: "france",
				Tenant: "mobile",
			}},
		}, {
			Description: "Invalid subnet (1)",
			Input:       gin.H{"192.0.2.1/38": "customer"},
			Output:      nil,
		}, {
			Description: "Invalid subnet (2)",
			Input:       gin.H{"192.0.2.1/255.0.255.0": "customer"},
			Output:      nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var got NetworkMap
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &got,
				ErrorUnused: true,
				Metadata:    nil,
				DecodeHook: mapstructure.ComposeDecodeHookFunc(
					NetworkMapUnmarshallerHook(),
				),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			decoder.Decode(tc.Input)
			if diff := helpers.Diff(got, tc.Output); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestDefaultConfiguration(t *testing.T) {
	config := DefaultConfiguration()
	config.Kafka.Topic = "flow"
	if err := helpers.Validate.Struct(config); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}
