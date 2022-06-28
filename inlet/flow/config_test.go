// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"strings"
	"testing"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

func TestDecodeConfiguration(t *testing.T) {
	cases := []struct {
		Name     string
		From     interface{}
		Source   interface{}
		Expected interface{}
	}{
		{
			Name: "from empty configuration",
			From: Configuration{},
			Source: map[string]interface{}{
				"inputs": []map[string]interface{}{
					{
						"type":    "udp",
						"decoder": "netflow",
						"listen":  "192.0.2.1:2055",
						"workers": 3,
					},
				},
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}},
			},
		}, {
			Name: "from existing configuration",
			From: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config:  udp.DefaultConfiguration(),
				}},
			},
			Source: map[string]interface{}{
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"type":    "udp",
						"decoder": "netflow",
						"listen":  "192.0.2.1:2055",
						"workers": 3,
					},
				},
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}},
			},
		}, {
			Name: "change type",
			From: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config:  udp.DefaultConfiguration(),
				}},
			},
			Source: map[string]interface{}{
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"type":  "file",
						"paths": []string{"file1", "file2"},
					},
				},
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &file.Configuration{
						Paths: []string{"file1", "file2"},
					},
				}},
			},
		}, {
			Name: "only set one item",
			From: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   2,
						QueueSize: 100,
						Listen:    "127.0.0.1:2055",
					},
				}},
			},
			Source: map[string]interface{}{
				"inputs": []map[string]interface{}{
					map[string]interface{}{
						"listen": "192.0.2.1:2055",
					},
				},
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   2,
						QueueSize: 100,
						Listen:    "192.0.2.1:2055",
					},
				}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got := tc.From

			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:           &got,
				ErrorUnused:      true,
				Metadata:         nil,
				WeaklyTypedInput: true,
				MatchName: func(mapKey, fieldName string) bool {
					key := strings.ToLower(strings.ReplaceAll(mapKey, "-", ""))
					field := strings.ToLower(fieldName)
					return key == field
				},
				DecodeHook: ConfigurationUnmarshalerHook(),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			if err := decoder.Decode(tc.Source); err != nil {
				t.Fatalf("Decode() error:\n%+v", err)
			}

			expected := tc.Expected
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestMarshalYAML(t *testing.T) {
	cfg := Configuration{
		Inputs: []InputConfiguration{
			{
				Decoder: "netflow",
				Config: &udp.Configuration{
					Listen:    "192.0.2.11:2055",
					QueueSize: 1000,
					Workers:   3,
				},
			},
		},
	}
	got, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error:\n%+v", err)
	}
	expected := `inputs:
- decoder: netflow
  listen: 192.0.2.11:2055
  queuesize: 1000
  receivebuffer: 0
  type: udp
  workers: 3
`
	if diff := helpers.Diff(strings.Split(string(got), "\n"), strings.Split(expected, "\n")); diff != "" {
		t.Fatalf("Marshal() (-got, +want):\n%s", diff)
	}
}
