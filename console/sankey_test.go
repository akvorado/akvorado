// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/query"
)

func TestGraphSankeyInputReverseDirection(t *testing.T) {
	input := graphSankeyHandlerInput{
		graphCommonHandlerInput: graphCommonHandlerInput{
			schema: schema.NewMock(t),
			Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
			End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
			Dimensions: query.Columns{
				query.NewColumn("ExporterName"),
				query.NewColumn("InIfProvider"),
			},
			Filter: query.NewFilter("DstCountry = 'FR' AND SrcCountry = 'US'"),
			Units:  "l3bps",
		},
		Bidirectional: true,
	}
	original1 := fmt.Sprintf("%+v", input)
	expected := graphSankeyHandlerInput{
		graphCommonHandlerInput: graphCommonHandlerInput{
			Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
			End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
			Dimensions: query.Columns{
				query.NewColumn("ExporterName"),
				query.NewColumn("OutIfProvider"),
			},
			Filter: query.NewFilter("SrcCountry = 'FR' AND DstCountry = 'US'"),
			Units:  "l3bps",
		},
		Bidirectional: true,
	}
	input.Filter.Validate(input.schema)
	expected.Filter.Validate(input.schema)
	query.Columns(input.Dimensions).Validate(input.schema)
	query.Columns(expected.Dimensions).Validate(input.schema)
	got := input.reverseDirection()
	original2 := fmt.Sprintf("%+v", input)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("reverseDirection() (-got, +want):\n%s", diff)
	}
	if original1 != original2 {
		t.Fatalf("reverseDirection() modified original to:\n-%s\n+%s", original1, original2)
	}
}

func TestSankeyQuerySQL(t *testing.T) {
	cases := []struct {
		Description string
		Pos         helpers.Pos
		Input       graphSankeyHandlerInput
		Expected    []templateQuery
	}{
		{
			Description: "two dimensions, no filters, l3 bps",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("ExporterName"),
					},
					Limit:  5,
					Filter: query.Filter{},
					Units:  "l3bps",
				},
			},
			Expected: []templateQuery{
				{
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "l3bps",
					},
					Template: `WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT 1 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC)`,
				},
			},
		}, {
			Description: "two dimensions, no filters, l3 bps, limitType by max",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("ExporterName"),
					},
					Limit:     5,
					LimitType: "max",
					Filter:    query.Filter{},
					Units:     "l3bps",
				},
			},
			Expected: []templateQuery{
				{
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "l3bps",
					},
					Template: `WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM ( SELECT SrcAS, ExporterName, {{ .Units }} AS sum_at_time FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ) GROUP BY SrcAS, ExporterName ORDER BY MAX(sum_at_time) DESC LIMIT 5)
SELECT 1 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC)`,
				},
			},
		}, {
			Description: "two dimensions, no filters, l2 bps",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("ExporterName"),
					},
					Limit:  5,
					Filter: query.Filter{},
					Units:  "l2bps",
				},
			},
			Expected: []templateQuery{
				{
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "l2bps",
					},
					Template: `WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT 1 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC)`,
				},
			},
		}, {
			Description: "two dimensions, no filters, pps",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("ExporterName"),
					},
					Limit:  5,
					Filter: query.Filter{},
					Units:  "pps",
				},
			},
			Expected: []templateQuery{
				{
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "pps",
					},
					Template: `WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT 1 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC)`,
				},
			},
		}, {
			Description: "two dimensions, with filter",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("ExporterName"),
					},
					Limit:  10,
					Filter: query.NewFilter("DstCountry = 'FR'"),
					Units:  "l3bps",
				},
			},
			Expected: []templateQuery{
				{
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "l3bps",
					},
					Template: `WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }} AND (DstCountry = 'FR')) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} AND (DstCountry = 'FR') GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 10)
SELECT 1 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (DstCountry = 'FR')
GROUP BY dimensions
ORDER BY xps DESC)`,
				},
			},
		}, {
			Description: "two dimensions, bidirectional",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("InIfProvider"),
					},
					Limit:  5,
					Filter: query.NewFilter("DstCountry = 'FR'"),
					Units:  "l3bps",
				},
				Bidirectional: true,
			},
			Expected: []templateQuery{
				{
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "l3bps",
					},
					Template: `WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }} AND (DstCountry = 'FR')) AS range,
 rows AS (SELECT SrcAS, InIfProvider FROM source WHERE {{ .Timefilter }} AND (DstCountry = 'FR') GROUP BY SrcAS, InIfProvider ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT 1 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(InIfProvider IN (SELECT InIfProvider FROM rows), InIfProvider, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (DstCountry = 'FR')
GROUP BY dimensions
ORDER BY xps DESC)`,
				}, {
					Context: inputContext{
						Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
						End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
						Points: 20,
						Units:  "l3bps",
					},
					Template: `SELECT 2 AS axis, * FROM (
SELECT
 {{ .Units }}/range AS xps,
 [if(DstAS IN (SELECT SrcAS FROM rows), concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???')), 'Other'),
  if(OutIfProvider IN (SELECT InIfProvider FROM rows), OutIfProvider, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (SrcCountry = 'FR')
GROUP BY dimensions
ORDER BY xps DESC)`,
				},
			},
		},
	}
	for _, tc := range cases {
		tc.Input.schema = schema.NewMock(t)
		if err := query.Columns(tc.Input.Dimensions).Validate(tc.Input.schema); err != nil {
			t.Fatalf("%sValidate() error:\n%+v", tc.Pos, err)
		}
		if err := tc.Input.Filter.Validate(tc.Input.schema); err != nil {
			t.Fatalf("%sValidate() error:\n%+v", tc.Pos, err)
		}
		t.Run(tc.Description, func(t *testing.T) {
			got, _ := tc.Input.toSQL()
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("%stoSQL (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}

func TestSankeyHandler(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	expectedSQL := []struct {
		Axis       uint8    `ch:"axis"`
		Xps        float64  `ch:"xps"`
		Dimensions []string `ch:"dimensions"`
	}{
		// [(random.randrange(100, 10000), x)
		//  for x in set([(random.choice(asn),
		//                 random.choice(providers),
		//                 random.choice(routers)) for x in range(30)])]
		{1, 9677, []string{"AS100", "Other", "router1"}},
		{1, 9472, []string{"AS300", "provider1", "Other"}},
		{1, 7593, []string{"AS300", "provider2", "router1"}},
		{1, 7234, []string{"AS200", "provider1", "Other"}},
		{1, 6006, []string{"AS100", "provider1", "Other"}},
		{1, 5988, []string{"Other", "provider1", "Other"}},
		{1, 4675, []string{"AS200", "provider3", "Other"}},
		{1, 4348, []string{"AS200", "Other", "router2"}},
		{1, 3999, []string{"AS100", "provider3", "Other"}},
		{1, 3978, []string{"AS100", "provider3", "router2"}},
		{1, 3623, []string{"Other", "Other", "router1"}},
		{1, 3080, []string{"AS300", "provider3", "router2"}},
		{1, 2915, []string{"AS300", "Other", "router1"}},
		{1, 2623, []string{"AS100", "provider1", "router1"}},
		{1, 2482, []string{"AS200", "provider2", "router2"}},
		{1, 2234, []string{"AS100", "provider2", "Other"}},
		{1, 1360, []string{"AS200", "Other", "router1"}},
		{1, 975, []string{"AS300", "Other", "Other"}},
		{1, 717, []string{"AS200", "provider3", "router2"}},
		{1, 621, []string{"Other", "Other", "Other"}},
		{1, 159, []string{"Other", "provider1", "router1"}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/graph/sankey",
			JSONInput: helpers.M{
				"start":      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				"end":        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				"dimensions": []string{"SrcAS", "InIfProvider", "ExporterName"},
				"limit":      10,
				"filter":     "DstCountry = 'FR'",
				"units":      "l3bps",
			},
			JSONOutput: helpers.M{
				// Raw data
				"rows": [][]string{
					{"AS100", "Other", "router1"},
					{"AS300", "provider1", "Other"},
					{"AS300", "provider2", "router1"},
					{"AS200", "provider1", "Other"},
					{"AS100", "provider1", "Other"},
					{"Other", "provider1", "Other"},
					{"AS200", "provider3", "Other"},
					{"AS200", "Other", "router2"},
					{"AS100", "provider3", "Other"},
					{"AS100", "provider3", "router2"},
					{"Other", "Other", "router1"},
					{"AS300", "provider3", "router2"},
					{"AS300", "Other", "router1"},
					{"AS100", "provider1", "router1"},
					{"AS200", "provider2", "router2"},
					{"AS100", "provider2", "Other"},
					{"AS200", "Other", "router1"},
					{"AS300", "Other", "Other"},
					{"AS200", "provider3", "router2"},
					{"Other", "Other", "Other"},
					{"Other", "provider1", "router1"},
				},
				"xps": []int{
					9677,
					9472,
					7593,
					7234,
					6006,
					5988,
					4675,
					4348,
					3999,
					3978,
					3623,
					3080,
					2915,
					2623,
					2482,
					2234,
					1360,
					975,
					717,
					621,
					159,
				},
				"axis": []int{
					1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
					1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
				},
				"axis-names": map[int]string{1: "Direct"},
				// For graph
				"nodes": []helpers.M{
					{"name": "SrcAS: AS100", "axis": 1},
					{"name": "InIfProvider: Other", "axis": 1},
					{"name": "ExporterName: router1", "axis": 1},
					{"name": "SrcAS: AS300", "axis": 1},
					{"name": "InIfProvider: provider1", "axis": 1},
					{"name": "ExporterName: Other", "axis": 1},
					{"name": "InIfProvider: provider2", "axis": 1},
					{"name": "SrcAS: AS200", "axis": 1},
					{"name": "SrcAS: Other", "axis": 1},
					{"name": "InIfProvider: provider3", "axis": 1},
					{"name": "ExporterName: router2", "axis": 1},
				},
				"links": []helpers.M{
					{
						"source": "InIfProvider: provider1", "target": "ExporterName: Other",
						"xps": 9472 + 7234 + 6006 + 5988, "axis": 1,
					},
					{
						"source": "InIfProvider: Other", "target": "ExporterName: router1",
						"xps": 9677 + 3623 + 2915 + 1360, "axis": 1,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: Other",
						"xps": 9677, "axis": 1,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: provider1",
						"xps": 9472, "axis": 1,
					},
					{
						"source": "InIfProvider: provider3", "target": "ExporterName: Other",
						"xps": 4675 + 3999, "axis": 1,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider1",
						"xps": 6006 + 2623, "axis": 1,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider3",
						"xps": 3999 + 3978, "axis": 1,
					},
					{
						"source": "InIfProvider: provider3", "target": "ExporterName: router2",
						"xps": 3978 + 3080 + 717, "axis": 1,
					},
					{
						"source": "InIfProvider: provider2", "target": "ExporterName: router1",
						"xps": 7593, "axis": 1,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: provider2",
						"xps": 7593, "axis": 1,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider1",
						"xps": 7234, "axis": 1,
					},
					{
						"source": "SrcAS: Other", "target": "InIfProvider: provider1",
						"xps": 5988 + 159, "axis": 1,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: Other",
						"xps": 4348 + 1360, "axis": 1,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider3",
						"xps": 4675 + 717, "axis": 1,
					},
					{
						"source": "InIfProvider: Other", "target": "ExporterName: router2",
						"xps": 4348, "axis": 1,
					},
					{
						"source": "SrcAS: Other", "target": "InIfProvider: Other",
						"xps": 3623 + 621, "axis": 1,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: Other",
						"xps": 2915 + 975, "axis": 1,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: provider3",
						"xps": 3080, "axis": 1,
					},
					{
						"source": "InIfProvider: provider1", "target": "ExporterName: router1",
						"xps": 2623 + 159, "axis": 1,
					},
					{
						"source": "InIfProvider: provider2", "target": "ExporterName: router2",
						"xps": 2482, "axis": 1,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider2",
						"xps": 2482, "axis": 1,
					},
					{
						"source": "InIfProvider: provider2", "target": "ExporterName: Other",
						"xps": 2234, "axis": 1,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider2",
						"xps": 2234, "axis": 1,
					},
					{
						"source": "InIfProvider: Other", "target": "ExporterName: Other",
						"xps": 975 + 621, "axis": 1,
					},
				},
			},
		},
	})
}

func TestSankeyHandlerBidirectional(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	expectedSQL := []struct {
		Axis       uint8    `ch:"axis"`
		Xps        float64  `ch:"xps"`
		Dimensions []string `ch:"dimensions"`
	}{
		// Forward direction (axis 1): SrcAS, InIfProvider
		{1, 9000, []string{"AS100", "provider1"}},
		{1, 7000, []string{"AS200", "provider1"}},
		{1, 5000, []string{"AS100", "provider2"}},
		{1, 3000, []string{"Other", "provider1"}},
		// Reverse direction (axis 2): DstAS, OutIfProvider
		{2, 8000, []string{"AS300", "provider1"}},
		{2, 4000, []string{"AS300", "provider3"}},
		{2, 2000, []string{"AS100", "Other"}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/graph/sankey",
			JSONInput: helpers.M{
				"start":         time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				"end":           time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				"dimensions":    []string{"SrcAS", "InIfProvider"},
				"limit":         10,
				"filter":        "DstCountry = 'FR'",
				"units":         "l3bps",
				"bidirectional": true,
			},
			JSONOutput: helpers.M{
				"rows": [][]string{
					{"AS100", "provider1"},
					{"AS200", "provider1"},
					{"AS100", "provider2"},
					{"Other", "provider1"},
					{"AS300", "provider1"},
					{"AS300", "provider3"},
					{"AS100", "Other"},
				},
				"xps":        []int{9000, 7000, 5000, 3000, 8000, 4000, 2000},
				"axis":       []int{1, 1, 1, 1, 2, 2, 2},
				"axis-names": map[int]string{1: "Direct", 2: "Reverse"},
				"nodes": []helpers.M{
					{"name": "SrcAS: AS100", "axis": 1},
					{"name": "InIfProvider: provider1", "axis": 1},
					{"name": "SrcAS: AS200", "axis": 1},
					{"name": "InIfProvider: provider2", "axis": 1},
					{"name": "SrcAS: Other", "axis": 1},
					{"name": "DstAS: AS300", "axis": 2},
					{"name": "OutIfProvider: provider1", "axis": 2},
					{"name": "OutIfProvider: provider3", "axis": 2},
					{"name": "DstAS: AS100", "axis": 2},
					{"name": "OutIfProvider: Other", "axis": 2},
				},
				"links": []helpers.M{
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider1",
						"xps": 9000, "axis": 1,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider1",
						"xps": 7000, "axis": 1,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider2",
						"xps": 5000, "axis": 1,
					},
					{
						"source": "SrcAS: Other", "target": "InIfProvider: provider1",
						"xps": 3000, "axis": 1,
					},
					{
						"source": "DstAS: AS300", "target": "OutIfProvider: provider1",
						"xps": 8000, "axis": 2,
					},
					{
						"source": "DstAS: AS300", "target": "OutIfProvider: provider3",
						"xps": 4000, "axis": 2,
					},
					{
						"source": "DstAS: AS100", "target": "OutIfProvider: Other",
						"xps": 2000, "axis": 2,
					},
				},
			},
		},
	})
}
