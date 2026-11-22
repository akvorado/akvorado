// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"akvorado/common/helpers"
	sb "akvorado/common/sqlbuilder"
)

func testConfiguration() Configuration {
	config := DefaultConfiguration()
	config.Start = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	config.End = time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	config.Fraction = 0.5
	config.Anonymization = AnonymizationConfiguration{
		IPv4Prefix: netip.MustParsePrefix("98.0.0.0/8"),
		IPv6Prefix: netip.MustParsePrefix("3fff::/20"),
	}
	return config
}

func TestStreamQuery(t *testing.T) {
	config := testConfiguration()
	config.Fraction = 1
	got := config.streamQuery(0.25)
	expected := `
SELECT TimeReceived, InIfBoundary, OutIfBoundary, EType, Proto,
       SrcAddr, DstAddr, SrcNetPrefix, DstNetPrefix, SrcPort, DstPort, Bytes, Packets
FROM flows
WHERE TimeReceived >= toDateTime('2026-08-01 00:00:00', 'UTC')
  AND TimeReceived < toDateTime('2026-08-08 00:00:00', 'UTC')
  AND murmurHash3_32(TimeReceived, SrcPort, DstPort) < 1073741824
  AND ((InIfBoundary = 'external' AND OutIfBoundary != 'external')
    OR (OutIfBoundary = 'external' AND InIfBoundary != 'external'))
LIMIT 2000000`
	if !got.Matches(expected) {
		t.Fatalf("streamQuery() == %s, expected %s", got, expected)
	}
}

func TestQueriesAvoidSensitiveColumns(t *testing.T) {
	config := testConfiguration()
	queries := map[string]string{
		"stream":    config.streamQuery(0.5).String(),
		"prefixes":  config.prefixQuery().String(),
		"protocols": config.protocolPortQuery().String(),
		"rate":      config.rateQuery().String(),
	}
	// The model must never learn these.
	sensitive := []string{
		"IfName", "IfDescription", "IfSpeed", "IfConnectivity", "IfProvider",
		"ASPath", "1stAS", "2ndAS", "3rdAS", "Communities", "ForwardingStatus",
		"ExporterAddress", "ExporterName",
		"NetName", "NetRole", "NetSite", "NetRegion", "NetTenant",
		"Country", "GeoCity", "GeoState", "SamplingRate",
	}
	for name, query := range queries {
		for _, column := range sensitive {
			if strings.Contains(query, column) {
				t.Errorf("query %s reads sensitive column %s:\n%s", name, column, query)
			}
		}
		// The local AS number identifies the network. Only the prefix query
		// reads the AS number, once, for the remote side.
		expected := 0
		if name == "prefixes" {
			expected = 1
		}
		for _, column := range []string{"SrcAS", "DstAS"} {
			if got := strings.Count(query, column); got != expected {
				t.Errorf("query %s reads %s %d times, expected %d:\n%s",
					name, column, got, expected, query)
			}
		}
	}
}

func TestRemoteOrLocal(t *testing.T) {
	cases := []struct {
		Remote   bool
		Expected string
	}{
		{true, "SELECT if(InIfBoundary = 'external', SrcAS, DstAS)"},
		{false, "SELECT if(InIfBoundary = 'external', DstAS, SrcAS)"},
	}
	for _, tc := range cases {
		got := sb.Select(remoteOrLocal(tc.Remote, "SrcAS", "DstAS"))
		if !got.Matches(tc.Expected) {
			t.Errorf("remoteOrLocal(%v) == %s, expected %s", tc.Remote, got, tc.Expected)
		}
	}
}

func TestExporterFilter(t *testing.T) {
	config := testConfiguration()
	config.Exporters = []string{"router1", "router2"}
	query := config.rateQuery().String()
	if !strings.Contains(query, "ExporterName IN ('router1', 'router2')") {
		t.Fatalf("rateQuery() misses the exporter filter:\n%s", query)
	}
}

func TestSampleFilter(t *testing.T) {
	cases := []struct {
		Fraction float64
		Expected uint64
	}{
		{0.75, 3221225472},
		{0.5, 2147483648},
		{0.1, 429496729},
	}
	for _, tc := range cases {
		got := sb.Select(sampleFilter(tc.Fraction))
		expected := fmt.Sprintf(
			"SELECT murmurHash3_32(TimeReceived, SrcPort, DstPort) < %d",
			tc.Expected)
		if !got.Matches(expected) {
			t.Errorf("sampleFilter(%g) == %s, expected %s", tc.Fraction, got, expected)
		}
	}

	// Without sampling, there is no condition.
	config := testConfiguration()
	config.Fraction = 1
	if query := config.rateQuery().String(); strings.Contains(query, "murmurHash3_32") {
		t.Errorf("rateQuery() with fraction 1 samples flows:\n%s", query)
	}
}

func TestBuckets(t *testing.T) {
	cases := []struct {
		Fraction float64
		PerHour  int
	}{
		{0.01, 1},
		{1. / 12, 1},
		{0.1, 2},
		{0.25, 3},
		{1, 12},
	}
	for _, tc := range cases {
		config := testConfiguration()
		config.Fraction = tc.Fraction
		buckets := config.buckets()
		if diff := helpers.Diff(len(buckets), 7*24*tc.PerHour); diff != "" {
			t.Errorf("buckets() for fraction %g (-got, +want):\n%s", tc.Fraction, diff)
		}
		perHour := map[time.Time]int{}
		for i, bucket := range buckets {
			if i > 0 && !buckets[i-1].Before(bucket) {
				t.Errorf("buckets() for fraction %g not sorted: %s, %s", tc.Fraction, buckets[i-1], bucket)
			}
			if bucket.Before(config.Start) || !bucket.Before(config.End) || bucket.Minute()%5 != 0 {
				t.Errorf("buckets() for fraction %g: unexpected %s", tc.Fraction, bucket)
			}
			perHour[bucket.Truncate(time.Hour)]++
		}
		for hour, count := range perHour {
			if count != tc.PerHour {
				t.Errorf("buckets() for fraction %g: %d buckets at %s", tc.Fraction, count, hour)
			}
		}
	}

	// A shorter range, not aligned on an hour, gets the same buckets.
	config := testConfiguration()
	config.Fraction = 0.25
	week := config.buckets()
	config.Start = time.Date(2026, 8, 2, 10, 20, 0, 0, time.UTC)
	config.End = time.Date(2026, 8, 2, 14, 40, 0, 0, time.UTC)
	expected := []time.Time{}
	for _, bucket := range week {
		if bucket.Add(bucketDuration).After(config.Start) && bucket.Before(config.End) {
			expected = append(expected, bucket)
		}
	}
	if diff := helpers.Diff(config.buckets(), expected); diff != "" {
		t.Errorf("buckets() for a shorter range (-got, +want):\n%s", diff)
	}
}

func TestBucketFilter(t *testing.T) {
	config := testConfiguration()
	config.Fraction = 0.01
	config.End = config.Start.Add(2 * time.Hour)
	buckets := config.buckets()
	fraction := 0.005
	got := sb.Select(config.commonFilter(fraction))
	expected := fmt.Sprintf(`
SELECT TimeReceived >= toDateTime('2026-08-01 00:00:00', 'UTC')
  AND TimeReceived < toDateTime('2026-08-01 02:00:00', 'UTC')
  AND toStartOfFiveMinutes(TimeReceived) IN (toDateTime('%s', 'UTC'), toDateTime('%s', 'UTC'))
  AND murmurHash3_32(TimeReceived, SrcPort, DstPort) < %d
  AND ((InIfBoundary = 'external' AND OutIfBoundary != 'external')
    OR (OutIfBoundary = 'external' AND InIfBoundary != 'external'))`,
		buckets[0].Format(time.DateTime), buckets[1].Format(time.DateTime),
		uint64(fraction*12*(1<<32)))
	if !got.Matches(expected) {
		t.Fatalf("commonFilter() == %s, expected %s", got, expected)
	}
}
