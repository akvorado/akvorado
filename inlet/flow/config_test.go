// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"strings"
	"testing"

	"akvorado/common/helpers/yaml"
	"akvorado/common/pb"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
	"akvorado/inlet/flow/input/file"
	"akvorado/inlet/flow/input/udp"
)

func TestDecodeConfiguration(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "from empty configuration",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
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
					Decoder: pb.RawFlow_DECODER_NETFLOW,
					Config: &udp.Configuration{
						Workers: 3,
						Listen:  "192.0.2.1:2055",
					},
					UseSrcAddrForExporterAddr: true,
				}, {
					Decoder: pb.RawFlow_DECODER_SFLOW,
					Config: &udp.Configuration{
						Workers: 3,
						Listen:  "192.0.2.1:6343",
					},
					UseSrcAddrForExporterAddr: false,
				}},
			},
		}, {
			Description: "ignore queue-size",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
						{
							"type":       "udp",
							"decoder":    "sflow",
							"listen":     "192.0.2.1:6343",
							"workers":    3,
							"queue-size": 1000,
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: pb.RawFlow_DECODER_SFLOW,
					Config: &udp.Configuration{
						Workers: 3,
						Listen:  "192.0.2.1:6343",
					},
					UseSrcAddrForExporterAddr: false,
				}},
			},
		},
		{
			Description: "from existing configuration",
			Initial: func() any {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: pb.RawFlow_DECODER_NETFLOW,
						Config:  udp.DefaultConfiguration(),
					}, {
						Decoder: pb.RawFlow_DECODER_SFLOW,
						Config:  udp.DefaultConfiguration(),
					}},
				}
			},
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
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
					Decoder: pb.RawFlow_DECODER_NETFLOW,
					Config: &udp.Configuration{
						Workers: 3,
						Listen:  "192.0.2.1:2055",
					},
				}, {
					Decoder: pb.RawFlow_DECODER_SFLOW,
					Config: &udp.Configuration{
						Workers: 3,
						Listen:  "192.0.2.1:6343",
					},
				}},
			},
		},
		{
			Description: "change type",
			Initial: func() any {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: pb.RawFlow_DECODER_NETFLOW,
						Config:  udp.DefaultConfiguration(),
					}},
				}
			},
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
						{
							"type":  "file",
							"paths": []string{"file1", "file2"},
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: pb.RawFlow_DECODER_NETFLOW,
					Config: &file.Configuration{
						Paths: []string{"file1", "file2"},
					},
				}},
			},
		},
		{
			Description: "only set one item",
			Initial: func() any {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder:         pb.RawFlow_DECODER_NETFLOW,
						TimestampSource: pb.RawFlow_TS_INPUT,
						Config: &udp.Configuration{
							Workers: 2,
							Listen:  "127.0.0.1:2055",
						},
					}},
				}
			},
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
						{
							"listen": "192.0.2.1:2055",
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder: pb.RawFlow_DECODER_NETFLOW,
					Config: &udp.Configuration{
						Workers: 2,
						Listen:  "192.0.2.1:2055",
					},
				}},
			},
		},
		{
			Description: "incorrect decoder",
			Initial: func() any {
				return Configuration{}
			},
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
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
		{
			Description: "netflow timestamp source netflow-packet",
			Initial: func() any {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: pb.RawFlow_DECODER_NETFLOW,
						Config: &udp.Configuration{
							Workers: 2,
							Listen:  "127.0.0.1:2055",
						},
					}},
				}
			},
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
						{
							"timestamp-source": "netflow-packet",
							"listen":           "192.0.2.1:2055",
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder:         pb.RawFlow_DECODER_NETFLOW,
					TimestampSource: pb.RawFlow_TS_NETFLOW_PACKET,
					Config: &udp.Configuration{
						Workers: 2,
						Listen:  "192.0.2.1:2055",
					},
				}},
			},
		},
		{
			Description: "netflow timestamp source netflow-first-switched",
			Initial: func() any {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: pb.RawFlow_DECODER_NETFLOW,
						Config: &udp.Configuration{
							Workers: 2,
							Listen:  "127.0.0.1:2055",
						},
					}},
				}
			},
			Configuration: func() any {
				return helpers.M{
					"inputs": []helpers.M{
						{
							"timestamp-source": "netflow-first-switched",
							"listen":           "192.0.2.1:2055",
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder:         pb.RawFlow_DECODER_NETFLOW,
					TimestampSource: pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED,
					Config: &udp.Configuration{
						Workers: 2,
						Listen:  "192.0.2.1:2055",
					},
				}},
			},
		},
		{
			Description: "netflow with decapsulation of VXLAN",
			Initial: func() any {
				return Configuration{
					Inputs: []InputConfiguration{{
						Decoder: pb.RawFlow_DECODER_NETFLOW,
						Config: &udp.Configuration{
							Workers: 2,
							Listen:  "127.0.0.1:2055",
						},
					}},
				}
			},
			Configuration: func() any {
				return gin.H{
					"inputs": []gin.H{
						{
							"decapsulation-protocol": "vxlan",
						},
					},
				}
			},
			Expected: Configuration{
				Inputs: []InputConfiguration{{
					Decoder:               pb.RawFlow_DECODER_NETFLOW,
					TimestampSource:       pb.RawFlow_TS_INPUT,
					DecapsulationProtocol: pb.RawFlow_DECAP_VXLAN,
					Config: &udp.Configuration{
						Workers: 2,
						Listen:  "127.0.0.1:2055",
					},
				}},
			},
		},
	})
}

func TestMarshalYAML(t *testing.T) {
	cfg := Configuration{
		Inputs: []InputConfiguration{
			{
				Decoder:         pb.RawFlow_DECODER_NETFLOW,
				TimestampSource: pb.RawFlow_TS_NETFLOW_FIRST_SWITCHED,
				Config: &udp.Configuration{
					Listen:  "192.0.2.11:2055",
					Workers: 3,
				},
			}, {
				Decoder:               pb.RawFlow_DECODER_SFLOW,
				DecapsulationProtocol: pb.RawFlow_DECAP_SRV6,
				Config: &udp.Configuration{
					Listen:  "192.0.2.11:6343",
					Workers: 3,
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
    - decapsulationprotocol: none
      decoder: netflow
      listen: 192.0.2.11:2055
      ratelimit: 0
      receivebuffer: 0
      timestampsource: netflow-first-switched
      type: udp
      usesrcaddrforexporteraddr: false
      workers: 3
    - decapsulationprotocol: srv6
      decoder: sflow
      listen: 192.0.2.11:6343
      ratelimit: 0
      receivebuffer: 0
      timestampsource: input
      type: udp
      usesrcaddrforexporteraddr: true
      workers: 3
`
	if diff := helpers.Diff(strings.Split(string(got), "\n"), strings.Split(expected, "\n")); diff != "" {
		t.Fatalf("Marshal() (-got, +want):\n%s", diff)
	}
}
