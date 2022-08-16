// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

func TestDecodeConfiguration(t *testing.T) {
	cases := []struct {
		Name     string
		From     func() interface{}
		Source   func() interface{}
		Expected interface{}
	}{
		{
			Name: "from empty configuration",
			From: func() interface{} { return Configuration{} },
			Source: func() interface{} {
				return gin.H{
					"inputs": []gin.H{
						{
							"type":    "udp",
							"decoder": "netflow",
							"listen":  "192.0.2.1:2055",
							"workers": 3,
						}, {
							"type":    "udp",
							"decoder": "sflow",
							"listen":  "192.0.2.1:6343",
							"workers": 3,
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}, {
					Decoder: "sflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:6343",
					},
				}},
			},
		}, {
			Name: "from existing configuration",
			From: func() interface{} {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: "netflow",
						Config:  udp.DefaultConfiguration(),
					}, {
						Decoder: "sflow",
						Config:  udp.DefaultConfiguration(),
					}},
				}
			},
			Source: func() interface{} {
				return gin.H{
					"inputs": []gin.H{
						{
							"type":    "udp",
							"decoder": "netflow",
							"listen":  "192.0.2.1:2055",
							"workers": 3,
						}, {
							"type":    "udp",
							"decoder": "sflow",
							"listen":  "192.0.2.1:6343",
							"workers": 3,
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: "netflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:2055",
					},
				}, {
					Decoder: "sflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:6343",
					},
				}},
			},
		}, {
			Name: "change type",
			From: func() interface{} {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: "netflow",
						Config:  udp.DefaultConfiguration(),
					}},
				}
			},
			Source: func() interface{} {
				return gin.H{
					"inputs": []gin.H{
						{
							"type":  "file",
							"paths": []string{"file1", "file2"},
						},
					},
				}
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
			From: func() interface{} {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: "netflow",
						Config: &udp.Configuration{
							Workers:   2,
							QueueSize: 100,
							Listen:    "127.0.0.1:2055",
						},
					}},
				}
			},
			Source: func() interface{} {
				return gin.H{
					"inputs": []gin.H{
						{
							"listen": "192.0.2.1:2055",
						},
					},
				}
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
		for _, viaYAML := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s (from YAML: %v)", tc.Name, viaYAML), func(t *testing.T) {
				var source interface{}
				if viaYAML {
					// Encode and decode with YAML
					out, err := yaml.Marshal(tc.Source())
					if err != nil {
						t.Fatalf("yaml.Marshal() error:\n%+v", err)
					}
					if err := yaml.Unmarshal(out, &source); err != nil {
						t.Fatalf("yaml.Unmarshal() error:\n%+v", err)
					}
				} else {
					// Just use as is
					source = tc.Source()
				}
				got := tc.From()

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
					DecodeHook: ConfigurationUnmarshallerHook(),
				})
				if err != nil {
					t.Fatalf("NewDecoder() error:\n%+v", err)
				}
				if err := decoder.Decode(source); err != nil {
					t.Fatalf("Decode() error:\n%+v", err)
				}

				if diff := helpers.Diff(got, tc.Expected); diff != "" {
					t.Fatalf("Decode() (-got, +want):\n%s", diff)
				}
			})
		}
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
			}, {
				Decoder: "sflow",
				Config: &udp.Configuration{
					Listen:    "192.0.2.11:6343",
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
- decoder: sflow
  listen: 192.0.2.11:6343
  queuesize: 1000
  receivebuffer: 0
  type: udp
  workers: 3
`
	if diff := helpers.Diff(strings.Split(string(got), "\n"), strings.Split(expected, "\n")); diff != "" {
		t.Fatalf("Marshal() (-got, +want):\n%s", diff)
	}
}
