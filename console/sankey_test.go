// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/query"
)

func TestSankeyQuerySQL(t *testing.T) {
	cases := []struct {
		Description string
		Pos         helpers.Pos
		Input       graphSankeyHandlerInput
		Expected    string
	}{
		{
			Description: "two dimensions, no filters, l3 bps",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput{
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
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":20,"units":"l3bps"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC
{{ end }}`,
		}, {
			Description: "two dimensions, no filters, l3 bps, limitType by max",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{
						query.NewColumn("SrcAS"),
						query.NewColumn("ExporterName"),
					},
					Limit:     5,
					LimitType: "Max",
					Filter:    query.Filter{},
					Units:     "l3bps",
				},
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":20,"units":"l3bps"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM ( SELECT SrcAS, ExporterName, MAX(Bytes) AS max_bytes_at_time FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName, {{ .Timefilter }} ) GROUP BY SrcAS, ExporterName ORDER BY MAX(max_bytes_at_time) DESC LIMIT 5)
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC
{{ end }}`,
		}, {
			Description: "two dimensions, no filters, l2 bps",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput{
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
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":20,"units":"l2bps"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC
{{ end }}
`,
		}, {
			Description: "two dimensions, no filters, pps",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput{
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
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":20,"units":"pps"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 5)
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY dimensions
ORDER BY xps DESC
{{ end }}`,
		}, {
			Description: "two dimensions, with filter",
			Pos:         helpers.Mark(),
			Input: graphSankeyHandlerInput{
				graphCommonHandlerInput{
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
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":20,"units":"l3bps"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM source WHERE {{ .Timefilter }} AND (DstCountry = 'FR')) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM source WHERE {{ .Timefilter }} AND (DstCountry = 'FR') GROUP BY SrcAS, ExporterName ORDER BY {{ .Units }} DESC LIMIT 10)
SELECT
 {{ .Units }}/range AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (DstCountry = 'FR')
GROUP BY dimensions
ORDER BY xps DESC
{{ end }}`,
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
		tc.Expected = strings.ReplaceAll(tc.Expected, "@@", "`")
		t.Run(tc.Description, func(t *testing.T) {
			got, _ := tc.Input.toSQL()
			if diff := helpers.Diff(strings.Split(strings.TrimSpace(got), "\n"),
				strings.Split(strings.TrimSpace(tc.Expected), "\n")); diff != "" {
				t.Errorf("%stoSQL (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}

func TestSankeyHandler(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	expectedSQL := []struct {
		Xps        float64  `ch:"xps"`
		Dimensions []string `ch:"dimensions"`
	}{
		// [(random.randrange(100, 10000), x)
		//  for x in set([(random.choice(asn),
		//                 random.choice(providers),
		//                 random.choice(routers)) for x in range(30)])]
		{9677, []string{"AS100", "Other", "router1"}},
		{9472, []string{"AS300", "provider1", "Other"}},
		{7593, []string{"AS300", "provider2", "router1"}},
		{7234, []string{"AS200", "provider1", "Other"}},
		{6006, []string{"AS100", "provider1", "Other"}},
		{5988, []string{"Other", "provider1", "Other"}},
		{4675, []string{"AS200", "provider3", "Other"}},
		{4348, []string{"AS200", "Other", "router2"}},
		{3999, []string{"AS100", "provider3", "Other"}},
		{3978, []string{"AS100", "provider3", "router2"}},
		{3623, []string{"Other", "Other", "router1"}},
		{3080, []string{"AS300", "provider3", "router2"}},
		{2915, []string{"AS300", "Other", "router1"}},
		{2623, []string{"AS100", "provider1", "router1"}},
		{2482, []string{"AS200", "provider2", "router2"}},
		{2234, []string{"AS100", "provider2", "Other"}},
		{1360, []string{"AS200", "Other", "router1"}},
		{975, []string{"AS300", "Other", "Other"}},
		{717, []string{"AS200", "provider3", "router2"}},
		{621, []string{"Other", "Other", "Other"}},
		{159, []string{"Other", "provider1", "router1"}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/graph/sankey",
			JSONInput: gin.H{
				"start":      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				"end":        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				"dimensions": []string{"SrcAS", "InIfProvider", "ExporterName"},
				"limit":      10,
				"filter":     "DstCountry = 'FR'",
				"units":      "l3bps",
			},
			JSONOutput: gin.H{
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
				// For graph
				"nodes": []string{
					"SrcAS: AS100",
					"InIfProvider: Other",
					"ExporterName: router1",
					"SrcAS: AS300",
					"InIfProvider: provider1",
					"ExporterName: Other",
					"InIfProvider: provider2",
					"SrcAS: AS200",
					"SrcAS: Other",
					"InIfProvider: provider3",
					"ExporterName: router2",
				},
				"links": []gin.H{
					{
						"source": "InIfProvider: provider1", "target": "ExporterName: Other",
						"xps": 9472 + 7234 + 6006 + 5988,
					},
					{
						"source": "InIfProvider: Other", "target": "ExporterName: router1",
						"xps": 9677 + 3623 + 2915 + 1360,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: Other",
						"xps": 9677,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: provider1",
						"xps": 9472,
					},
					{
						"source": "InIfProvider: provider3", "target": "ExporterName: Other",
						"xps": 4675 + 3999,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider1",
						"xps": 6006 + 2623,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider3",
						"xps": 3999 + 3978,
					},
					{
						"source": "InIfProvider: provider3", "target": "ExporterName: router2",
						"xps": 3978 + 3080 + 717,
					},
					{
						"source": "InIfProvider: provider2", "target": "ExporterName: router1",
						"xps": 7593,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: provider2",
						"xps": 7593,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider1",
						"xps": 7234,
					},
					{
						"source": "SrcAS: Other", "target": "InIfProvider: provider1",
						"xps": 5988 + 159,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: Other",
						"xps": 4348 + 1360,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider3",
						"xps": 4675 + 717,
					},
					{
						"source": "InIfProvider: Other", "target": "ExporterName: router2",
						"xps": 4348,
					},
					{
						"source": "SrcAS: Other", "target": "InIfProvider: Other",
						"xps": 3623 + 621,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: Other",
						"xps": 2915 + 975,
					},
					{
						"source": "SrcAS: AS300", "target": "InIfProvider: provider3",
						"xps": 3080,
					},
					{
						"source": "InIfProvider: provider1", "target": "ExporterName: router1",
						"xps": 2623 + 159,
					},
					{
						"source": "InIfProvider: provider2", "target": "ExporterName: router2",
						"xps": 2482,
					},
					{
						"source": "SrcAS: AS200", "target": "InIfProvider: provider2",
						"xps": 2482,
					},
					{
						"source": "InIfProvider: provider2", "target": "ExporterName: Other",
						"xps": 2234,
					},
					{
						"source": "SrcAS: AS100", "target": "InIfProvider: provider2",
						"xps": 2234,
					},
					{
						"source": "InIfProvider: Other", "target": "ExporterName: Other",
						"xps": 975 + 621,
					},
				},
			},
		},
	})
}
