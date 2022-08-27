// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"net/netip"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"akvorado/common/helpers"
)

func TestSubnetMapUnmarshalHook(t *testing.T) {
	var nilMap map[string]string
	cases := []struct {
		Description string
		Input       interface{}
		Tests       map[string]string
		Error       bool
		YAML        interface{}
	}{
		{
			Description: "nil",
			Input:       nilMap,
			Tests: map[string]string{
				"::ffff:203.0.113.1": "",
			},
		}, {
			Description: "empty",
			Input:       gin.H{},
			Tests: map[string]string{
				"::ffff:203.0.113.1": "",
			},
		}, {
			Description: "IPv4 subnet",
			Input:       gin.H{"203.0.113.0/24": "customer1"},
			Tests: map[string]string{
				"::ffff:203.0.113.18": "customer1",
				"::ffff:203.0.113.16": "customer1",
				"203.0.113.16":        "",
				"::ffff:203.0.1.1":    "",
				"203.0.1.1":           "",
				"2001:db8:1::12":      "",
			},
		}, {
			Description: "IPv4 IP",
			Input:       gin.H{"203.0.113.1": "customer1"},
			Tests: map[string]string{
				"::ffff:203.0.113.1": "customer1",
				"2001:db8:1::12":     "",
			},
			YAML: gin.H{"203.0.113.1/32": "customer1"},
		}, {
			Description: "IPv6 subnet",
			Input:       gin.H{"2001:db8:1::/64": "customer2"},
			Tests: map[string]string{
				"2001:db8:1::1": "customer2",
				"2001:db8:1::2": "customer2",
				"2001:db8:2::2": "",
			},
		}, {
			Description: "IPv6-mapped-IPv4 subnet",
			Input:       gin.H{"::ffff:203.0.113.0/120": "customer2"},
			Tests: map[string]string{
				"::ffff:203.0.113.10": "customer2",
				"::ffff:203.0.112.10": "",
			},
			YAML: gin.H{"203.0.113.0/24": "customer2"},
		}, {
			Description: "IPv6 IP",
			Input:       gin.H{"2001:db8:1::1": "customer2"},
			Tests: map[string]string{
				"2001:db8:1::1": "customer2",
				"2001:db8:1::2": "",
				"2001:db8:2::2": "",
			},
			YAML: gin.H{"2001:db8:1::1/128": "customer2"},
		}, {
			Description: "Invalid subnet (1)",
			Input:       gin.H{"192.0.2.1/38": "customer"},
			Error:       true,
		}, {
			Description: "Invalid subnet (2)",
			Input:       gin.H{"192.0.2.1/255.0.255.0": "customer"},
			Error:       true,
		}, {
			Description: "Invalid subnet (3)",
			Input:       gin.H{"2001:db8::/1000": "customer"},
			Error:       true,
		}, {
			Description: "Invalid IP",
			Input:       gin.H{"200.33.300.1": "customer"},
			Error:       true,
		}, {
			Description: "Random key",
			Input:       gin.H{"kfgdjgkfj": "customer"},
			Error:       true,
		}, {
			Description: "Single value",
			Input:       "customer",
			Tests: map[string]string{
				"::ffff:203.0.113.4": "customer",
				"2001:db8::1":        "customer",
			},
			YAML: map[string]string{
				"::/0": "customer",
			},
		},
	}
	for _, tc := range cases {
		if tc.YAML == nil {
			if tc.Error {
				tc.YAML = map[string]string{}
			} else {
				tc.YAML = tc.Input
			}
		}
		t.Run(tc.Description, func(t *testing.T) {
			var tree helpers.SubnetMap[string]
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &tree,
				ErrorUnused: true,
				Metadata:    nil,
				DecodeHook:  helpers.SubnetMapUnmarshallerHook[string](),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			err = decoder.Decode(tc.Input)
			if err != nil && !tc.Error {
				t.Fatalf("Decode() error:\n%+v", err)
			} else if err == nil && tc.Error {
				t.Fatal("Decode() did not return an error")
			}
			got := map[string]string{}
			for k := range tc.Tests {
				v, _ := tree.Lookup(netip.MustParseAddr(k))
				got[k] = v
			}
			if diff := helpers.Diff(got, tc.Tests); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}

			// Try to unmarshal with YAML
			buf, err := yaml.Marshal(tree)
			if err != nil {
				t.Fatalf("yaml.Marshal() error:\n%+v", err)
			}
			got = map[string]string{}
			if err := yaml.Unmarshal(buf, &got); err != nil {
				t.Fatalf("yaml.Unmarshal() error:\n%+v", err)
			}
			if diff := helpers.Diff(got, tc.YAML); diff != "" {
				t.Fatalf("MarshalYAML() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestSubnetMapUnmarshalHookWithMapValue(t *testing.T) {
	type SomeStruct struct {
		Blip string
		Blop string
	}
	cases := []struct {
		Description string
		Input       gin.H
		Expected    gin.H
	}{
		{
			Description: "single value",
			Input: gin.H{
				"blip": "some",
				"blop": "thing",
			},
			Expected: gin.H{
				"::/0": gin.H{
					"Blip": "some",
					"Blop": "thing",
				},
			},
		}, {
			Description: "proper map",
			Input: gin.H{
				"::/0": gin.H{
					"blip": "some",
					"blop": "thing",
				},
				"203.0.113.14": gin.H{
					"blip": "other",
					"blop": "stuff",
				},
			},
			Expected: gin.H{
				"::/0": gin.H{
					"Blip": "some",
					"Blop": "thing",
				},
				"203.0.113.14/32": gin.H{
					"Blip": "other",
					"Blop": "stuff",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var tree helpers.SubnetMap[SomeStruct]
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &tree,
				ErrorUnused: true,
				Metadata:    nil,
				DecodeHook:  helpers.SubnetMapUnmarshallerHook[SomeStruct](),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			err = decoder.Decode(tc.Input)
			if err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}
			if diff := helpers.Diff(tree.ToMap(), tc.Expected); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestToMap(t *testing.T) {
	input := helpers.MustNewSubnetMap(map[string]string{
		"2001:db8::/64":        "hello",
		"::ffff:192.0.2.0/120": "bye",
	})
	got := input.ToMap()
	expected := map[string]string{
		"2001:db8::/64": "hello",
		"192.0.2.0/24":  "bye",
	}
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("ToMap() (-got, +want):\n%s", diff)
	}
}
