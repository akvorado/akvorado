// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package static

import (
	"context"
	"net/netip"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/metadata/provider"
)

func TestStaticProvider(t *testing.T) {
	config := Configuration{
		Exporters: helpers.MustNewSubnetMap(map[string]ExporterConfiguration{
			"2001:db8:1::/48": {
				Exporter: provider.Exporter{
					Name: "nodefault",
				},
				IfIndexes: map[uint]provider.Interface{
					10: {
						Name:        "Gi10",
						Description: "10th interface",
						Speed:       1000,
					},
					11: {
						Name:        "Gi11",
						Description: "11th interface",
						Speed:       1000,
					},
				},
			},
			"2001:db8:2::/48": {
				Exporter: provider.Exporter{
					Name: "default",
				},
				Default: provider.Interface{
					Name:        "Default0",
					Description: "Default interface",
					Speed:       1000,
				},
				IfIndexes: map[uint]provider.Interface{
					10: {
						Name:        "Gi10",
						Description: "10th interface",
						Speed:       1000,
					},
				},
			},
			"2001:db8:3::/48": {
				Exporter: provider.Exporter{
					Name:   "default with metadata",
					Region: "eu",
					Role:   "peering",
					Tenant: "mine",
					Site:   "par",
					Group:  "blue",
				},
				Default: provider.Interface{
					Name:        "Default0",
					Description: "Default interface",
					Speed:       1000,
				},
				IfIndexes: map[uint]provider.Interface{
					10: {
						Name:         "Gi10",
						Description:  "10th interface",
						Speed:        1000,
						Provider:     "transit101",
						Connectivity: "transit",
						Boundary:     schema.InterfaceBoundaryExternal,
					},
				},
			},
			"2001:db8:4::/48": {
				Exporter: provider.Exporter{
					Name: "nodefault skip",
				},
				IfIndexes: map[uint]provider.Interface{
					10: {
						Name:         "Gi10",
						Description:  "10th interface",
						Speed:        1000,
						Provider:     "transit101",
						Connectivity: "transit",
						Boundary:     schema.InterfaceBoundaryExternal,
					},
				},
				SkipMissingInterfaces: true,
			},
		}),
	}

	var got []provider.Update
	r := reporter.NewMock(t)
	p, _ := config.New(r, func(update provider.Update) {
		got = append(got, update)
	})

	p.Query(context.Background(), &provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:1::10"),
		IfIndexes:  []uint{9, 10, 11},
	})
	p.Query(context.Background(), &provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
		IfIndexes:  []uint{9, 10, 11},
	})
	p.Query(context.Background(), &provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:3::10"),
		IfIndexes:  []uint{10},
	})
	query := provider.BatchQuery{
		ExporterIP: netip.MustParseAddr("2001:db8:4::10"),
		IfIndexes:  []uint{9, 10, 11},
	}
	err := p.Query(context.Background(), &query)

	expected := []provider.Update{
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:1::10"),
				IfIndex:    9,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "nodefault",
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:1::10"),
				IfIndex:    10,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "nodefault",
				},
				Interface: provider.Interface{
					Name:        "Gi10",
					Description: "10th interface",
					Speed:       1000,
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:1::10"),
				IfIndex:    11,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "nodefault",
				},
				Interface: provider.Interface{
					Name:        "Gi11",
					Description: "11th interface",
					Speed:       1000,
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
				IfIndex:    9,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "default",
				},
				Interface: provider.Interface{
					Name:        "Default0",
					Description: "Default interface",
					Speed:       1000,
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
				IfIndex:    10,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "default",
				},
				Interface: provider.Interface{
					Name:        "Gi10",
					Description: "10th interface",
					Speed:       1000,
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:2::10"),
				IfIndex:    11,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "default",
				},
				Interface: provider.Interface{
					Name:        "Default0",
					Description: "Default interface",
					Speed:       1000,
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:3::10"),
				IfIndex:    10,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name:   "default with metadata",
					Region: "eu",
					Role:   "peering",
					Tenant: "mine",
					Site:   "par",
					Group:  "blue",
				},
				Interface: provider.Interface{
					Name:         "Gi10",
					Description:  "10th interface",
					Speed:        1000,
					Provider:     "transit101",
					Connectivity: "transit",
					Boundary:     schema.InterfaceBoundaryExternal,
				},
			},
		},
		{
			Query: provider.Query{
				ExporterIP: netip.MustParseAddr("2001:db8:4::10"),
				IfIndex:    10,
			},
			Answer: provider.Answer{
				Exporter: provider.Exporter{
					Name: "nodefault skip",
				},
				Interface: provider.Interface{
					Name:         "Gi10",
					Description:  "10th interface",
					Speed:        1000,
					Provider:     "transit101",
					Connectivity: "transit",
					Boundary:     schema.InterfaceBoundaryExternal,
				},
			},
		},
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(query.IfIndexes, []uint{9, 11}); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(err, provider.ErrSkipProvider); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
}
