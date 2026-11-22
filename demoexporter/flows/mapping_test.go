// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/demoexporter/flowmodel"
)

// testModelPath writes a small model to a temporary file.
func testModelPath(t *testing.T) string {
	t.Helper()
	model := flowmodel.NewMock(t)
	path := filepath.Join(t.TempDir(), "model.bin")
	output, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error:\n%+v", err)
	}
	defer output.Close()
	if err := model.Save(output); err != nil {
		t.Fatalf("Save() error:\n%+v", err)
	}
	return path
}

// testInterface returns an interface configuration.
func testInterface(description string, boundary schema.InterfaceBoundary, asn uint32) InterfaceConfiguration {
	return InterfaceConfiguration{
		Description: description, Speed: 10000,
		Boundary: boundary,
		ASN:      asn,
	}
}

// testConfiguration returns a configuration with two exporters.
func testConfiguration(t *testing.T) Configuration {
	config := DefaultConfiguration()
	config.Model = testModelPath(t)
	config.Exporter = "exporter1"
	config.Target = "127.0.0.1:2055"
	config.Exporters = map[string]ExporterConfiguration{
		"exporter1": {
			SamplingRate: 1000,
			Interfaces: map[int]InterfaceConfiguration{
				10: testInterface("Transit: Telia", schema.InterfaceBoundaryExternal, 1299),
				20: testInterface("Core", schema.InterfaceBoundaryInternal, 0),
			},
		},
		"exporter2": {
			SamplingRate: 1000,
			Interfaces: map[int]InterfaceConfiguration{
				11: testInterface("PNI: Google", schema.InterfaceBoundaryExternal, 15169),
				21: testInterface("Core", schema.InterfaceBoundaryInternal, 0),
			},
		},
	}
	config.Mapping = []MappingRuleConfiguration{
		{
			ASN: []uint32{15169},
			Assign: []AssignConfiguration{
				{Exporter: "exporter2", Interface: 11, Weight: 95},
				{Exporter: "exporter1", Interface: 10, Weight: 5},
			},
		}, {
			Assign: []AssignConfiguration{
				{Exporter: "exporter1", Interface: 10, Weight: 1},
			},
		},
	}
	return config
}

func TestNewMappingErrors(t *testing.T) {
	cases := []struct {
		Description string
		Mutate      func(*Configuration)
	}{
		{"unknown exporter", func(config *Configuration) {
			config.Mapping[1].Assign[0].Exporter = "unknown"
		}},
		{"unknown interface", func(config *Configuration) {
			config.Mapping[1].Assign[0].Interface = 42
		}},
		{"internal interface", func(config *Configuration) {
			config.Mapping[1].Assign[0].Interface = 20
		}},
		{"exporter without flows", func(config *Configuration) {
			config.Exporter = "exporter2"
			config.Mapping[0].Assign = config.Mapping[0].Assign[1:]
		}},
		{"no rule for any AS number", func(config *Configuration) {
			config.Mapping = config.Mapping[:1]
		}},
		{"no internal interface", func(config *Configuration) {
			delete(config.Exporters["exporter1"].Interfaces, 20)
		}},
	}
	for _, tc := range cases {
		config := testConfiguration(t)
		tc.Mutate(&config)
		if _, err := newMapping(&config); err == nil {
			t.Errorf("newMapping() did not error on %s", tc.Description)
		}
	}
}

func TestMappingAssign(t *testing.T) {
	config := testConfiguration(t)
	mapping, err := newMapping(&config)
	if err != nil {
		t.Fatalf("newMapping() error:\n%+v", err)
	}
	r := rand.New(rand.NewPCG(0, 0))
	counts := map[string]int{}
	const total = 10000
	for range total {
		flow := flowmodel.Flow{
			Key:       r.Uint64(),
			RemoteASN: 15169,
		}
		exporter, ifIndex, ok := mapping.assign(&flow)
		if !ok {
			t.Fatalf("assign() did not assign the flow")
		}
		expected := map[string]int{"exporter1": 10, "exporter2": 11}[exporter]
		if diff := helpers.Diff(ifIndex, expected); diff != "" {
			t.Fatalf("assign() interface on %s (-got, +want):\n%s", exporter, diff)
		}
		counts[exporter]++
	}
	// 95% of the Google flows go to exporter2.
	share := float64(counts["exporter2"]) / total
	if share < 0.93 || share > 0.97 {
		t.Fatalf("assign() gives %g%% of Google flows to exporter2, expected 95%%", share*100)
	}

	// Flows from another AS always use the catch-all rule.
	for range 100 {
		flow := flowmodel.Flow{Key: r.Uint64(), RemoteASN: 2906}
		exporter, ifIndex, _ := mapping.assign(&flow)
		if diff := helpers.Diff([]any{exporter, ifIndex}, []any{"exporter1", 10}); diff != "" {
			t.Fatalf("assign() for AS2906 (-got, +want):\n%s", diff)
		}
	}

	// The assignment is deterministic.
	flow := flowmodel.Flow{Key: 12345, RemoteASN: 15169}
	firstExporter, firstIf, _ := mapping.assign(&flow)
	secondExporter, secondIf, _ := mapping.assign(&flow)
	if diff := helpers.Diff([]any{firstExporter, firstIf}, []any{secondExporter, secondIf}); diff != "" {
		t.Fatalf("assign() is not deterministic (-got, +want):\n%s", diff)
	}
}

func TestOtherInterface(t *testing.T) {
	config := testConfiguration(t)
	config.Exporters["exporter1"].Interfaces[11] = testInterface("IX: AMSIX", schema.InterfaceBoundaryExternal, 0)
	mapping, err := newMapping(&config)
	if err != nil {
		t.Fatalf("newMapping() error:\n%+v", err)
	}
	r := rand.New(rand.NewPCG(0, 0))
	for range 100 {
		cases := []struct {
			Boundary schema.InterfaceBoundary
			Expected int
		}{
			{schema.InterfaceBoundaryInternal, 20},
			// With no interface for the boundary, fall back to an internal one.
			{schema.InterfaceBoundaryUndefined, 20},
			// The external interface carrying the flow is not used twice.
			{schema.InterfaceBoundaryExternal, 11},
		}
		for _, tc := range cases {
			got := mapping.otherInterface("exporter1", tc.Boundary, 10, r.Uint64())
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("otherInterface(%s) (-got, +want):\n%s", tc.Boundary, diff)
			}
		}
	}
}

func TestFleetPartition(t *testing.T) {
	config1 := testConfiguration(t)
	config2 := config1
	config2.Exporter = "exporter2"
	r := reporter.NewMock(t)
	c1, err := New(r, config1, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	c2, err := New(r, config2, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	now := time.Date(2022, 3, 18, 15, 0, 0, 0, time.UTC)
	total, split := 0, 0
	for i := range 50 {
		when := now.Add(time.Duration(i) * time.Second)
		flows1 := c1.generate(when)
		flows2 := c2.generate(when)
		all := c1.sampler.Generate(config1.Seed, when, nil)
		total += len(all)
		split += len(flows1) + len(flows2)
		// The two instances split the flows without overlap: the catch-all
		// mapping rule assigns everything, so the split covers all the flows.
		if diff := helpers.Diff(len(flows1)+len(flows2), len(all)); diff != "" {
			t.Fatalf("generate() split at %s (-got, +want):\n%s", when, diff)
		}
	}
	if total == 0 {
		t.Fatalf("generate() did not produce any flow")
	}
	if diff := helpers.Diff(split, total); diff != "" {
		t.Fatalf("generate() partition (-got, +want):\n%s", diff)
	}
}
