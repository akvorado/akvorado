// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"

	"akvorado/common/helpers"
)

func TestGraphQuerySQL(t *testing.T) {
	cases := []struct {
		Description string
		Input       graphHandlerInput
		Expected    string
	}{
		{
			Description: "no dimensions, no filters, bps",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter:     queryFilter{},
				Units:      "l3bps",
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Bytes*SamplingRate*8/slot) AS xps,
 emptyArrayString() AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY time, dimensions
ORDER BY time`,
		}, {
			Description: "no dimensions, no filters, l2 bps",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter:     queryFilter{},
				Units:      "l2bps",
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM((Bytes+18*Packets)*SamplingRate*8/slot) AS xps,
 emptyArrayString() AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY time, dimensions
ORDER BY time`,
		}, {
			Description: "no dimensions, no filters, pps",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter:     queryFilter{},
				Units:      "pps",
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Packets*SamplingRate/slot) AS xps,
 emptyArrayString() AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY time, dimensions
ORDER BY time`,
		}, {
			Description: "no dimensions",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter:     queryFilter{"DstCountry = 'FR' AND SrcCountry = 'US'"},
				Units:      "l3bps",
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Bytes*SamplingRate*8/slot) AS xps,
 emptyArrayString() AS dimensions
FROM {table}
WHERE {timefilter} AND (DstCountry = 'FR' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time`,
		}, {
			Description: "no filters",
			Input: graphHandlerInput{
				Start:  time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points: 100,
				Limit:  20,
				Dimensions: []queryColumn{
					queryColumnExporterName,
					queryColumnInIfProvider,
				},
				Filter: queryFilter{},
				Units:  "l3bps",
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot,
 rows AS (SELECT ExporterName, InIfProvider FROM {table} WHERE {timefilter} GROUP BY ExporterName, InIfProvider ORDER BY SUM(Bytes) DESC LIMIT 20)
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Bytes*SamplingRate*8/slot) AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY time, dimensions
ORDER BY time`,
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

func TestGraphHandler(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	base := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	expectedSQL := []struct {
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{
		{base, 1000, []string{"router1", "provider1"}},
		{base, 2000, []string{"router1", "provider2"}},
		{base, 1200, []string{"router2", "provider2"}},
		{base, 1100, []string{"router2", "provider3"}},
		{base, 1900, []string{"Other", "Other"}},
		{base.Add(time.Minute), 500, []string{"router1", "provider1"}},
		{base.Add(time.Minute), 5000, []string{"router1", "provider2"}},
		{base.Add(time.Minute), 900, []string{"router2", "provider4"}},
		{base.Add(time.Minute), 100, []string{"Other", "Other"}},
		{base.Add(2 * time.Minute), 100, []string{"router1", "provider1"}},
		{base.Add(2 * time.Minute), 3000, []string{"router1", "provider2"}},
		{base.Add(2 * time.Minute), 100, []string{"router2", "provider4"}},
		{base.Add(2 * time.Minute), 100, []string{"Other", "Other"}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/graph",
			JSONInput: gin.H{
				"start":      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				"end":        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				"points":     100,
				"limit":      20,
				"dimensions": []string{"ExporterName", "InIfProvider"},
				"filter":     "DstCountry = 'FR' AND SrcCountry = 'US'",
				"units":      "l3bps",
			},
			JSONOutput: gin.H{
				// Sorted by sum of bps
				"rows": [][]string{
					{"router1", "provider2"}, // 10000
					{"router1", "provider1"}, // 1600
					{"router2", "provider2"}, // 1200
					{"router2", "provider3"}, // 1100
					{"router2", "provider4"}, // 1000
					{"Other", "Other"},       // 2100
				},
				"t": []string{
					"2009-11-10T23:00:00Z",
					"2009-11-10T23:01:00Z",
					"2009-11-10T23:02:00Z",
				},
				"points": [][]int{
					{2000, 5000, 3000},
					{1000, 500, 100},
					{1200, 0, 0},
					{1100, 0, 0},
					{0, 900, 100},
					{1900, 100, 100},
				},
				"min": []int{
					2000,
					100,
					0,
					0,
					0,
					100,
				},
				"max": []int{
					5000,
					1000,
					1200,
					1100,
					900,
					1900,
				},
				"average": []int{
					3333,
					533,
					400,
					366,
					333,
					700,
				},
				"95th": []int{
					4000,
					750,
					600,
					550,
					500,
					1000,
				},
			},
		},
	})
}
