// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"testing"

	"github.com/mitchellh/mapstructure"

	"akvorado/common/helpers"
)

func TestNetworkNamesUnmarshalHook(t *testing.T) {
	cases := []struct {
		Description string
		Input       map[string]string
		Output      NetworkNames
	}{
		{
			Description: "nil",
			Input:       nil,
			Output:      NetworkNames{},
		}, {
			Description: "empty",
			Input:       map[string]string{},
			Output:      NetworkNames{},
		}, {
			Description: "IPv4 subnet",
			Input:       map[string]string{"203.0.113.0/24": "customer"},
			Output:      NetworkNames{"::ffff:203.0.113.0/120": "customer"},
		}, {
			Description: "IPv6 subnet",
			Input:       map[string]string{"2001:db8:1::/64": "customer"},
			Output:      NetworkNames{"2001:db8:1::/64": "customer"},
		}, {
			Description: "Invalid subnet (1)",
			Input:       map[string]string{"192.0.2.1/38": "customer"},
			Output:      nil,
		}, {
			Description: "Invalid subnet (2)",
			Input:       map[string]string{"192.0.2.1/255.0.255.0": "customer"},
			Output:      nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var got NetworkNames
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &got,
				ErrorUnused: true,
				Metadata:    nil,
				DecodeHook: mapstructure.ComposeDecodeHookFunc(
					NetworkNamesUnmarshalerHook(),
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
