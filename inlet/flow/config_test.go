// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

func TestDecodeConfiguration(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "from empty configuration",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"inputs": []gin.H{
						{
							"type":                           "udp",
							"decoder":                        "netflow",
							"listen":                         "192.0.2.1:2055",
							"workers":                        3,
							"use-src-addr-for-exporter-addr": true,
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
					UseSrcAddrForExporterAddr: true,
				}, {
					Decoder: "sflow",
					Config: &udp.Configuration{
						Workers:   3,
						QueueSize: 100000,
						Listen:    "192.0.2.1:6343",
					},
					UseSrcAddrForExporterAddr: false,
				}},
			},
		}, {
			Description: "from existing configuration",
			Initial: func() interface{} {
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
			Configuration: func() interface{} {
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
			Description: "change type",
			Initial: func() interface{} {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: "netflow",
						Config:  udp.DefaultConfiguration(),
					}},
				}
			},
			Configuration: func() interface{} {
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
			Description: "only set one item",
			Initial: func() interface{} {
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
			Configuration: func() interface{} {
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
		}, {
			Description: "incorrect decoder",
			Initial: func() interface{} {
				return Configuration{}
			},
			Configuration: func() interface{} {
				return gin.H{
					"inputs": []gin.H{
						{
							"type":    "avians",
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
			Error: true,
		},
	})
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
				UseSrcAddrForExporterAddr: true,
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
      usesrcaddrforexporteraddr: false
      workers: 3
    - decoder: sflow
      listen: 192.0.2.11:6343
      queuesize: 1000
      receivebuffer: 0
      type: udp
      usesrcaddrforexporteraddr: true
      workers: 3
ratelimit: 0
`
	if diff := helpers.Diff(strings.Split(string(got), "\n"), strings.Split(expected, "\n")); diff != "" {
		t.Fatalf("Marshal() (-got, +want):\n%s", diff)
	}
}
