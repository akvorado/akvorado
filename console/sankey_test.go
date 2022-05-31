package console

import (
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestSankeyQuerySQL(t *testing.T) {
	cases := []struct {
		Description string
		Input       sankeyHandlerInput
		Expected    string
	}{
		{
			Description: "two dimensions, no filters, bps",
			Input: sankeyHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Dimensions: []queryColumn{queryColumnSrcAS, queryColumnExporterName},
				Limit:      5,
				Filter:     queryFilter{},
				Units:      "bps",
			},
			Expected: `
WITH
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM {table} WHERE {timefilter}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM {table} WHERE {timefilter} GROUP BY SrcAS, ExporterName ORDER BY SUM(Bytes) DESC LIMIT 5)
SELECT
 SUM(Bytes*SamplingRate*8/range) AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY dimensions
ORDER BY xps DESC`,
		}, {
			Description: "two dimensions, no filters, pps",
			Input: sankeyHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Dimensions: []queryColumn{queryColumnSrcAS, queryColumnExporterName},
				Limit:      5,
				Filter:     queryFilter{},
				Units:      "pps",
			},
			Expected: `
WITH
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM {table} WHERE {timefilter}) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM {table} WHERE {timefilter} GROUP BY SrcAS, ExporterName ORDER BY SUM(Bytes) DESC LIMIT 5)
SELECT
 SUM(Packets*SamplingRate/range) AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY dimensions
ORDER BY xps DESC`,
		}, {
			Description: "two dimensions, with filter",
			Input: sankeyHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Dimensions: []queryColumn{queryColumnSrcAS, queryColumnExporterName},
				Limit:      10,
				Filter:     queryFilter{"DstCountry = 'FR'"},
				Units:      "bps",
			},
			Expected: `
WITH
 (SELECT MAX(TimeReceived) - MIN(TimeReceived) FROM {table} WHERE {timefilter} AND (DstCountry = 'FR')) AS range,
 rows AS (SELECT SrcAS, ExporterName FROM {table} WHERE {timefilter} AND (DstCountry = 'FR') GROUP BY SrcAS, ExporterName ORDER BY SUM(Bytes) DESC LIMIT 10)
SELECT
 SUM(Bytes*SamplingRate*8/range) AS xps,
 [if(SrcAS IN (SELECT SrcAS FROM rows), concat(toString(SrcAS), ': ', dictGetOrDefault('asns', 'name', SrcAS, '???')), 'Other'),
  if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')] AS dimensions
FROM {table}
WHERE {timefilter} AND (DstCountry = 'FR')
GROUP BY dimensions
ORDER BY xps DESC`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			got, _ := tc.Input.toSQL()
			if diff := helpers.Diff(strings.Split(got, "\n"), strings.Split(tc.Expected, "\n")); diff != "" {
				t.Errorf("toSQL (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestSankeyHandler(t *testing.T) {
	r := reporter.NewMock(t)
	ch, mockConn := clickhousedb.NewMock(t, r)
	h := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: ch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

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

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/sankey",
			JSONInput: gin.H{
				"start":      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				"end":        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				"dimensions": []string{"SrcAS", "InIfProvider", "ExporterName"},
				"limit":      10,
				"filter":     "DstCountry = 'FR'",
				"units":      "bps",
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
					"AS100",
					"Other InIfProvider",
					"router1",
					"AS300",
					"provider1",
					"Other ExporterName",
					"provider2",
					"AS200",
					"Other SrcAS",
					"provider3",
					"router2",
				},
				"links": []gin.H{
					{"source": "provider1", "target": "Other ExporterName", "xps": 9472 + 7234 + 6006 + 5988},
					{"source": "Other InIfProvider", "target": "router1", "xps": 9677 + 3623 + 2915 + 1360},
					{"source": "AS100", "target": "Other InIfProvider", "xps": 9677},
					{"source": "AS300", "target": "provider1", "xps": 9472},
					{"source": "provider3", "target": "Other ExporterName", "xps": 4675 + 3999},
					{"source": "AS100", "target": "provider1", "xps": 6006 + 2623},
					{"source": "AS100", "target": "provider3", "xps": 3999 + 3978},
					{"source": "provider3", "target": "router2", "xps": 3978 + 3080 + 717},
					{"source": "AS300", "target": "provider2", "xps": 7593},
					{"source": "provider2", "target": "router1", "xps": 7593},
					{"source": "AS200", "target": "provider1", "xps": 7234},
					{"source": "Other SrcAS", "target": "provider1", "xps": 5988 + 159},
					{"source": "AS200", "target": "Other InIfProvider", "xps": 4348 + 1360},
					{"source": "AS200", "target": "provider3", "xps": 4675 + 717},
					{"source": "Other InIfProvider", "target": "router2", "xps": 4348},
					{"source": "Other SrcAS", "target": "Other InIfProvider", "xps": 3623 + 621},
					{"source": "AS300", "target": "Other InIfProvider", "xps": 2915 + 975},
					{"source": "AS300", "target": "provider3", "xps": 3080},
					{"source": "provider1", "target": "router1", "xps": 2623 + 159},
					{"source": "AS200", "target": "provider2", "xps": 2482},
					{"source": "provider2", "target": "router2", "xps": 2482},
					{"source": "AS100", "target": "provider2", "xps": 2234},
					{"source": "provider2", "target": "Other ExporterName", "xps": 2234},
					{"source": "Other InIfProvider", "target": "Other ExporterName", "xps": 975 + 621},
				},
			},
		},
	})
}
