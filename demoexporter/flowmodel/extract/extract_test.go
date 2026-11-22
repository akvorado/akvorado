// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/demoexporter/flowmodel"
)

// setupFlowsTable creates a small flows table with the columns used
// by the extraction, including the alias columns for the prefixes.
func setupFlowsTable(t *testing.T, chComponent *clickhousedb.Component) {
	t.Helper()
	aliases := map[string]string{}
	for _, side := range []string{"Src", "Dst"} {
		aliases[side] = fmt.Sprintf(`CASE
 WHEN EType = 0x800 THEN concat(replaceRegexpOne(IPv6CIDRToRange(%sAddr, (96 + %sNetMask)::UInt8).1::String, '^::ffff:', ''), '/', %sNetMask::String)
 WHEN EType = 0x86dd THEN concat(IPv6CIDRToRange(%sAddr, %sNetMask).1::String, '/', %sNetMask::String)
 ELSE ''
END`, side, side, side, side, side, side)
	}
	create := fmt.Sprintf(`
CREATE TABLE flows (
 TimeReceived DateTime,
 ExporterName LowCardinality(String),
 SrcAddr IPv6,
 DstAddr IPv6,
 SrcNetMask UInt8,
 DstNetMask UInt8,
 SrcNetPrefix String ALIAS %s,
 DstNetPrefix String ALIAS %s,
 SrcAS UInt32,
 DstAS UInt32,
 InIfBoundary Enum8('undefined' = 0, 'external' = 1, 'internal' = 2),
 OutIfBoundary Enum8('undefined' = 0, 'external' = 1, 'internal' = 2),
 EType UInt32,
 Proto UInt32,
 SrcPort UInt16,
 DstPort UInt16,
 Bytes UInt64,
 Packets UInt64
) ENGINE = MergeTree() ORDER BY TimeReceived`, aliases["Src"], aliases["Dst"])
	if err := chComponent.Exec(t.Context(), create); err != nil {
		t.Fatalf("Exec() error:\n%+v", err)
	}

	// Two customers talk with Google over IPv4, one customer talks
	// with Netflix, a service receives traffic, one customer talks
	// with another customer. One internal flow and one transit flow should
	// be ignored.
	insert := `
INSERT INTO flows
 (TimeReceived, ExporterName, SrcAddr, DstAddr, SrcNetMask, DstNetMask, SrcAS, DstAS,
  InIfBoundary, OutIfBoundary, EType, Proto, SrcPort, DstPort, Bytes, Packets)
VALUES
 ('2026-08-03 10:00:00', 'router1', '::ffff:216.58.206.10', '::ffff:203.0.113.5', 24, 24, 15169, 64501, 'external', 'internal', 0x800, 6, 443, 50000, 4500, 3),
 ('2026-08-03 10:00:01', 'router1', '::ffff:216.58.206.11', '::ffff:203.0.113.6', 24, 24, 15169, 64501, 'external', 'internal', 0x800, 6, 443, 50001, 3000, 2),
 ('2026-08-03 10:00:02', 'router1', '::ffff:203.0.113.5', '::ffff:216.58.206.10', 24, 24, 64501, 15169, 'internal', 'external', 0x800, 6, 50000, 443, 600, 4),
 ('2026-08-03 10:00:03', 'router1', '::ffff:198.38.120.20', '::ffff:203.0.113.7', 23, 24, 2906, 64501, 'external', 'internal', 0x800, 6, 443, 50002, 15000, 10),
 ('2026-08-03 10:00:04', 'router1', '::ffff:198.51.100.1', '::ffff:192.0.2.10', 24, 24, 64502, 64501, 'external', 'internal', 0x800, 17, 5353, 53, 400, 1),
 ('2026-08-03 10:00:04', 'router1', '::ffff:203.0.113.9', '::ffff:203.0.113.5', 24, 24, 64501, 64501, 'external', 'internal', 0x800, 6, 443, 50003, 1500, 1),
 ('2026-08-03 10:00:05', 'router1', '::ffff:10.0.0.1', '::ffff:10.0.0.2', 24, 24, 64501, 64501, 'internal', 'internal', 0x800, 6, 1000, 2000, 100, 1),
 ('2026-08-03 10:00:06', 'router1', '::ffff:216.58.206.12', '::ffff:198.38.120.21', 24, 23, 15169, 2906, 'external', 'external', 0x800, 6, 443, 50004, 1500, 1)`
	if err := chComponent.Exec(t.Context(), insert); err != nil {
		t.Fatalf("Exec() error:\n%+v", err)
	}
}

func TestExtract(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r, false)
	setupFlowsTable(t, chComponent)

	config := DefaultConfiguration()
	config.Start = time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	config.End = time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	config.Fraction = 1
	config.MinCount = 1
	config.MaxFlows = 1000
	config.Anonymization = AnonymizationConfiguration{
		IPv4Prefix: netip.MustParsePrefix("98.0.0.0/8"),
		IPv6Prefix: netip.MustParsePrefix("3fff::/20"),
	}

	result, err := Extract(t.Context(), r, chComponent, config)
	if err != nil {
		t.Fatalf("Extract() error:\n%+v", err)
	}

	if diff := helpers.Diff(result.Flows, 6); diff != "" {
		t.Errorf("Extract() flows (-got, +want):\n%s", diff)
	}

	// Remote prefixes: Google and Netflix, plus the two catch-all
	// entries. The local prefix and the documentation prefix are not
	// part of them.
	remotes := map[string]uint32{}
	for _, info := range result.Metadata.RemotePrefixes {
		if info.Prefix.Bits() > 0 {
			remotes[info.Prefix.String()] = info.ASN
		}
	}
	expectedRemotes := map[string]uint32{
		"216.58.206.0/24": 15169,
		"198.38.120.0/23": 2906,
	}
	if diff := helpers.Diff(remotes, expectedRemotes); diff != "" {
		t.Errorf("Extract() remote prefixes (-got, +want):\n%s", diff)
	}

	// Local prefixes: both /24 are anonymized inside the target prefix.
	target := netip.MustParsePrefix("98.0.0.0/8")
	locals := 0
	for _, info := range result.Metadata.LocalPrefixes {
		if info.Prefix.Bits() == 0 {
			continue
		}
		locals++
		if !target.Contains(info.Prefix.Addr()) || info.Prefix.Bits() != 24 {
			t.Errorf("Extract() local prefix %s not anonymized", info.Prefix)
		}
	}
	if diff := helpers.Diff(locals, 2); diff != "" {
		t.Errorf("Extract() local prefixes (-got, +want):\n%s", diff)
	}

	// Examples: 6 flows, mostly distinct.
	var totalWeight float32
	for _, example := range result.Examples {
		totalWeight += example.Weight
	}
	if diff := helpers.Diff(totalWeight, float32(6)); diff != "" {
		t.Errorf("Extract() total weight (-got, +want):\n%s", diff)
	}

	// The rate profile only has flows on Monday at 10:00 UTC. The
	// other days use the same profile. The average is 1.
	expectedRates := [168]float32{}
	for hour := 10; hour < len(expectedRates); hour += 24 {
		expectedRates[hour] = 24
	}
	if diff := helpers.Diff(result.Metadata.RateProfile, expectedRates); diff != "" {
		t.Errorf("Extract() rate profile (-got, +want):\n%s", diff)
	}

	// Rare values are not learned: Netflix and UDP only have one flow.
	config.MinCount = 2
	rare, err := Extract(t.Context(), r, chComponent, config)
	if err != nil {
		t.Fatalf("Extract() error:\n%+v", err)
	}
	remotes = map[string]uint32{}
	for _, info := range rare.Metadata.RemotePrefixes {
		if info.Prefix.Bits() > 0 {
			remotes[info.Prefix.String()] = info.ASN
		}
	}
	if diff := helpers.Diff(remotes, map[string]uint32{"216.58.206.0/24": 15169}); diff != "" {
		t.Errorf("Extract() remote prefixes with MinCount (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(rare.Metadata.ProtoEtypes,
		[]flowmodel.ProtoEtype{{EType: 0x800, Proto: 6}}); diff != "" {
		t.Errorf("Extract() protocols with MinCount (-got, +want):\n%s", diff)
	}

	// The result can train a model end to end.
	model := flowmodel.New(result.Metadata, 0)
	if err := model.Train(result.Examples, flowmodel.TrainOptions{
		Epochs: 2, BatchSize: 16, LearningRate: 0.01, Workers: 1,
	}, nil); err != nil {
		t.Fatalf("Train() error:\n%+v", err)
	}
}
