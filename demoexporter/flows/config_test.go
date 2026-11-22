// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/demoexporter/snmp"
)

func TestConfigurationDecode(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "complete",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return map[string]any{
					"target": "127.0.0.1:2055",
					"exporters": map[string]any{
						"exporter1": map[string]any{
							"interfaces": map[string]any{
								"10": map[string]any{
									"description": "Transit: Telia",
									"boundary":    "external",
									"asn":         1299,
								},
								"20": map[string]any{
									"description": "Core",
									"boundary":    "internal",
									"speed":       100000,
								},
							},
						},
					},
					"mapping": []map[string]any{
						{
							"asn": []uint32{15169},
							"assign": []map[string]any{
								{"exporter": "exporter1", "interface": 10},
							},
						},
					},
				}
			},
			Expected: func() Configuration {
				expected := DefaultConfiguration()
				expected.Target = "127.0.0.1:2055"
				expected.Exporters = map[string]ExporterConfiguration{
					"exporter1": {
						SamplingRate: 1000,
						Interfaces: map[int]InterfaceConfiguration{
							10: {
								InterfaceConfiguration: snmp.InterfaceConfiguration{Description: "Transit: Telia"},
								Boundary:               schema.InterfaceBoundaryExternal,
								ASN:                    1299,
							},
							20: {
								InterfaceConfiguration: snmp.InterfaceConfiguration{Description: "Core", Speed: 100000},
								Boundary:               schema.InterfaceBoundaryInternal,
							},
						},
					},
				}
				expected.Mapping = []MappingRuleConfiguration{
					{
						ASN: []uint32{15169},
						Assign: []AssignConfiguration{
							{Exporter: "exporter1", Interface: 10, Weight: 1},
						},
					},
				}
				return expected
			}(),
		}, {
			Description: "missing fleet",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return map[string]any{
					"target": "127.0.0.1:2055",
				}
			},
			Error: true,
		}, {
			Description: "interface without boundary",
			Initial:     func() any { return DefaultConfiguration() },
			Configuration: func() any {
				return map[string]any{
					"target": "127.0.0.1:2055",
					"exporters": map[string]any{
						"exporter1": map[string]any{
							"interfaces": map[string]any{
								"10": map[string]any{"description": "Transit: Telia"},
							},
						},
					},
					"mapping": []map[string]any{
						{"assign": []map[string]any{{"exporter": "exporter1", "interface": 10}}},
					},
				}
			},
			Error: true,
		},
	})
}
