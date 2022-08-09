// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"

	"akvorado/common/helpers"
)

func TestGraphInputReverseDirection(t *testing.T) {
	input := graphHandlerInput{
		Start:  time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
		End:    time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
		Points: 100,
		Dimensions: []queryColumn{
			queryColumnExporterName,
			queryColumnInIfProvider,
		},
		Filter: queryFilter{
			Filter:        "DstCountry = 'FR' AND SrcCountry = 'US'",
			ReverseFilter: "SrcCountry = 'FR' AND DstCountry = 'US'",
		},
		Units: "l3bps",
	}
	original1 := fmt.Sprintf("%+v", input)
	expected := graphHandlerInput{
		Start:  time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
		End:    time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
		Points: 100,
		Dimensions: []queryColumn{
			queryColumnExporterName,
			queryColumnOutIfProvider,
		},
		Filter: queryFilter{
			Filter:        "SrcCountry = 'FR' AND DstCountry = 'US'",
			ReverseFilter: "DstCountry = 'FR' AND SrcCountry = 'US'",
		},
		Units: "l3bps",
	}
	got := input.reverseDirection()
	original2 := fmt.Sprintf("%+v", input)
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("reverseDirection() (-got, +want):\n%s", diff)
	}
	if original1 != original2 {
		t.Fatalf("reverseDirection() modified original to:\n-%s\n+%s", original1, original2)
	}
}

func TestGraphPreviousPeriod(t *testing.T) {
	const longForm = "Jan 2, 2006 at 15:04"
	cases := []struct {
		Start         string
		End           string
		ExpectedStart string
		ExpectedEnd   string
	}{
		{
			"Jan 2, 2020 at 15:04", "Jan 2, 2020 at 16:04",
			"Jan 2, 2020 at 14:04", "Jan 2, 2020 at 15:04",
		}, {
			"Jan 2, 2020 at 15:04", "Jan 2, 2020 at 16:34",
			"Jan 2, 2020 at 14:04", "Jan 2, 2020 at 15:34",
		}, {
			"Jan 2, 2020 at 15:04", "Jan 2, 2020 at 17:34",
			"Jan 1, 2020 at 15:04", "Jan 1, 2020 at 17:34",
		}, {
			"Jan 2, 2020 at 15:04", "Jan 3, 2020 at 17:34",
			"Jan 1, 2020 at 15:04", "Jan 2, 2020 at 17:34",
		}, {
			"Jan 10, 2020 at 15:04", "Jan 13, 2020 at 17:34",
			"Jan 3, 2020 at 15:04", "Jan 6, 2020 at 17:34",
		}, {
			"Jan 10, 2020 at 15:04", "Jan 15, 2020 at 17:34",
			"Jan 3, 2020 at 15:04", "Jan 8, 2020 at 17:34",
		}, {
			"Jan 10, 2020 at 15:04", "Jan 20, 2020 at 17:34",
			"Jan 3, 2020 at 15:04", "Jan 13, 2020 at 17:34",
		}, {
			"Feb 10, 2020 at 15:04", "Feb 25, 2020 at 17:34",
			"Jan 13, 2020 at 15:04", "Jan 28, 2020 at 17:34",
		}, {
			"Feb 10, 2020 at 15:04", "Mar 25, 2020 at 17:34",
			"Jan 13, 2020 at 15:04", "Feb 26, 2020 at 17:34",
		}, {
			"Feb 10, 2020 at 15:04", "Jul 25, 2020 at 17:34",
			"Feb 10, 2019 at 15:04", "Jul 25, 2019 at 17:34",
		}, {
			"Feb 10, 2019 at 15:04", "Jul 25, 2020 at 17:34",
			"Feb 10, 2018 at 15:04", "Jul 25, 2019 at 17:34",
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s to %s", tc.Start, tc.End), func(t *testing.T) {
			start, err := time.Parse(longForm, tc.Start)
			if err != nil {
				t.Fatalf("time.Parse(%q) error:\n%+v", tc.Start, err)
			}
			end, err := time.Parse(longForm, tc.End)
			if err != nil {
				t.Fatalf("time.Parse(%q) error:\n%+v", tc.End, err)
			}
			expectedStart, err := time.Parse(longForm, tc.ExpectedStart)
			if err != nil {
				t.Fatalf("time.Parse(%q) error:\n%+v", tc.ExpectedStart, err)
			}
			expectedEnd, err := time.Parse(longForm, tc.ExpectedEnd)
			if err != nil {
				t.Fatalf("time.Parse(%q) error:\n%+v", tc.ExpectedEnd, err)
			}
			input := graphHandlerInput{
				Start: start,
				End:   end,
				Dimensions: []queryColumn{
					queryColumnExporterAddress,
					queryColumnExporterName,
				},
			}
			got := input.previousPeriod()
			expected := graphHandlerInput{
				Start:      expectedStart,
				End:        expectedEnd,
				Dimensions: []queryColumn{},
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("previousPeriod() (-got, +want):\n%s", diff)
			}
		})
	}
}

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
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
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
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l2bps"}@@ }}
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}
`,
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
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"pps"}@@ }}
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
		}, {
			Description: "no dimensions",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter:     queryFilter{Filter: "DstCountry = 'FR' AND SrcCountry = 'US'"},
				Units:      "l3bps",
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }} AND (DstCountry = 'FR' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
		}, {
			Description: "no dimensions, escaped filter",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter:     queryFilter{Filter: "InIfDescription = '{{ hello }}' AND SrcCountry = 'US'"},
				Units:      "l3bps",
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }} AND (InIfDescription = '{{"{{"}} hello }}' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
		}, {
			Description: "no dimensions, reverse direction",
			Input: graphHandlerInput{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []queryColumn{},
				Filter: queryFilter{
					Filter:        "DstCountry = 'FR' AND SrcCountry = 'US'",
					ReverseFilter: "SrcCountry = 'FR' AND DstCountry = 'US'",
				},
				Units:         "l3bps",
				Bidirectional: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }} AND (DstCountry = 'FR' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 2 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }} AND (SrcCountry = 'FR' AND DstCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
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
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
WITH
 rows AS (SELECT ExporterName, InIfProvider FROM {{ .Table }} WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider ORDER BY SUM(Bytes) DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
		}, {
			Description: "no filters, reverse",
			Input: graphHandlerInput{
				Start:  time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points: 100,
				Limit:  20,
				Dimensions: []queryColumn{
					queryColumnExporterName,
					queryColumnInIfProvider,
				},
				Filter:        queryFilter{},
				Units:         "l3bps",
				Bidirectional: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
WITH
 rows AS (SELECT ExporterName, InIfProvider FROM {{ .Table }} WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider ORDER BY SUM(Bytes) DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 2 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, OutIfProvider) IN rows, [ExporterName, OutIfProvider], ['Other', 'Other']) AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}`,
		}, {
			Description: "no filters, previous period",
			Input: graphHandlerInput{
				Start:  time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points: 100,
				Limit:  20,
				Dimensions: []queryColumn{
					queryColumnExporterName,
					queryColumnInIfProvider,
				},
				Filter:         queryFilter{},
				Units:          "l3bps",
				PreviousPeriod: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps"}@@ }}
WITH
 rows AS (SELECT ExporterName, InIfProvider FROM {{ .Table }} WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider ORDER BY SUM(Bytes) DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }})
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-09T15:45:10Z","end":"2022-04-10T15:45:10Z","start-for-interval":"2022-04-10T15:45:10Z","points":100,"units":"l3bps"}@@ }}
SELECT 3 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} + INTERVAL 86400 second AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM {{ .Table }}
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }} + INTERVAL 86400 second
 TO {{ .TimefilterEnd }} + INTERVAL 1 second + INTERVAL 86400 second
 STEP {{ .Interval }})
{{ end }}`,
		},
	}
	for _, tc := range cases {
		tc.Expected = strings.ReplaceAll(tc.Expected, "@@", "`")
		t.Run(tc.Description, func(t *testing.T) {
			got := tc.Input.toSQL()
			if diff := helpers.Diff(strings.Split(strings.TrimSpace(got), "\n"),
				strings.Split(strings.TrimSpace(tc.Expected), "\n")); diff != "" {
				t.Errorf("toSQL (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestGraphHandler(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())
	base := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	// Single direction
	expectedSQL := []struct {
		Axis       uint8     `ch:"axis"`
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{
		{1, base, 1000, []string{"router1", "provider1"}},
		{1, base, 2000, []string{"router1", "provider2"}},
		{1, base, 1200, []string{"router2", "provider2"}},
		{1, base, 1100, []string{"router2", "provider3"}},
		{1, base, 1900, []string{"Other", "Other"}},
		{1, base.Add(time.Minute), 500, []string{"router1", "provider1"}},
		{1, base.Add(time.Minute), 5000, []string{"router1", "provider2"}},
		{1, base.Add(time.Minute), 900, []string{"router2", "provider4"}},
		{1, base.Add(time.Minute), 100, []string{"Other", "Other"}},
		{1, base.Add(2 * time.Minute), 100, []string{"router1", "provider1"}},
		{1, base.Add(2 * time.Minute), 3000, []string{"router1", "provider2"}},
		{1, base.Add(2 * time.Minute), 100, []string{"router2", "provider4"}},
		{1, base.Add(2 * time.Minute), 100, []string{"Other", "Other"}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	// Bidirectional
	expectedSQL = []struct {
		Axis       uint8     `ch:"axis"`
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{
		{1, base, 1000, []string{"router1", "provider1"}},
		{1, base, 2000, []string{"router1", "provider2"}},
		{1, base, 1200, []string{"router2", "provider2"}},
		{1, base, 1100, []string{"router2", "provider3"}},
		{1, base, 1900, []string{"Other", "Other"}},
		{1, base.Add(time.Minute), 500, []string{"router1", "provider1"}},
		{1, base.Add(time.Minute), 5000, []string{"router1", "provider2"}},
		{1, base.Add(time.Minute), 900, []string{"router2", "provider4"}},

		// Axes can be mixed. In reality, it seems they cannot
		// be interleaved, but ClickHouse documentation does
		// not say it is not possible.
		{2, base, 100, []string{"router1", "provider1"}},
		{2, base, 200, []string{"router1", "provider2"}},
		{2, base, 120, []string{"router2", "provider2"}},

		{1, base.Add(time.Minute), 100, []string{"Other", "Other"}},
		{1, base.Add(2 * time.Minute), 100, []string{"router1", "provider1"}},

		{2, base, 110, []string{"router2", "provider3"}},
		{2, base, 190, []string{"Other", "Other"}},
		{2, base.Add(time.Minute), 50, []string{"router1", "provider1"}},
		{2, base.Add(time.Minute), 500, []string{"router1", "provider2"}},

		{1, base.Add(2 * time.Minute), 3000, []string{"router1", "provider2"}},
		{1, base.Add(2 * time.Minute), 100, []string{"router2", "provider4"}},
		{1, base.Add(2 * time.Minute), 100, []string{"Other", "Other"}},

		{2, base.Add(time.Minute), 90, []string{"router2", "provider4"}},
		{2, base.Add(time.Minute), 10, []string{"Other", "Other"}},
		{2, base.Add(2 * time.Minute), 10, []string{"router1", "provider1"}},
		{2, base.Add(2 * time.Minute), 300, []string{"router1", "provider2"}},
		{2, base.Add(2 * time.Minute), 10, []string{"router2", "provider4"}},
		{2, base.Add(2 * time.Minute), 10, []string{"Other", "Other"}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	// Previous period
	expectedSQL = []struct {
		Axis       uint8     `ch:"axis"`
		Time       time.Time `ch:"time"`
		Xps        float64   `ch:"xps"`
		Dimensions []string  `ch:"dimensions"`
	}{
		{1, base, 1000, []string{"router1", "provider1"}},
		{1, base, 2000, []string{"router1", "provider2"}},
		{1, base, 1200, []string{"router2", "provider2"}},
		{1, base, 1100, []string{"router2", "provider3"}},
		{1, base, 1900, []string{"Other", "Other"}},
		{1, base.Add(time.Minute), 500, []string{"router1", "provider1"}},
		{1, base.Add(time.Minute), 5000, []string{"router1", "provider2"}},
		{1, base.Add(time.Minute), 900, []string{"router2", "provider4"}},
		{1, base.Add(time.Minute), 100, []string{"Other", "Other"}},
		{1, base.Add(2 * time.Minute), 100, []string{"router1", "provider1"}},
		{1, base.Add(2 * time.Minute), 3000, []string{"router1", "provider2"}},
		{1, base.Add(2 * time.Minute), 100, []string{"router2", "provider4"}},
		{1, base.Add(2 * time.Minute), 100, []string{"Other", "Other"}},

		{3, base, 8000, []string{}},
		{3, base.Add(time.Minute), 6000, []string{}},
		{3, base.Add(2 * time.Minute), 4500, []string{}},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			Description: "single direction",
			URL:         "/api/v0/console/graph",
			JSONInput: gin.H{
				"start":         time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				"end":           time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				"points":        100,
				"limit":         20,
				"dimensions":    []string{"ExporterName", "InIfProvider"},
				"filter":        "DstCountry = 'FR' AND SrcCountry = 'US'",
				"units":         "l3bps",
				"bidirectional": false,
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
					1200,
					1100,
					100,
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
				"axis": []int{
					1, 1, 1, 1, 1, 1,
				},
				"axis-names": map[int]string{
					1: "Direct",
				},
			},
		}, {
			Description: "bidirectional",
			URL:         "/api/v0/console/graph",
			JSONInput: gin.H{
				"start":         time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				"end":           time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				"points":        100,
				"limit":         20,
				"dimensions":    []string{"ExporterName", "InIfProvider"},
				"filter":        "DstCountry = 'FR' AND SrcCountry = 'US'",
				"units":         "l3bps",
				"bidirectional": true,
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

					{"router1", "provider2"}, // 1000
					{"router1", "provider1"}, // 160
					{"router2", "provider2"}, // 120
					{"router2", "provider3"}, // 110
					{"router2", "provider4"}, // 100
					{"Other", "Other"},       // 210
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

					{200, 500, 300},
					{100, 50, 10},
					{120, 0, 0},
					{110, 0, 0},
					{0, 90, 10},
					{190, 10, 10},
				},
				"min": []int{
					2000,
					100,
					1200,
					1100,
					100,
					100,

					200,
					10,
					120,
					110,
					10,
					10,
				},
				"max": []int{
					5000,
					1000,
					1200,
					1100,
					900,
					1900,

					500,
					100,
					120,
					110,
					90,
					190,
				},
				"average": []int{
					3333,
					533,
					400,
					366,
					333,
					700,

					333,
					53,
					40,
					36,
					33,
					70,
				},
				"95th": []int{
					4000,
					750,
					600,
					550,
					500,
					1000,

					400,
					75,
					60,
					55,
					50,
					100,
				},
				"axis": []int{
					1, 1, 1, 1, 1, 1,
					2, 2, 2, 2, 2, 2,
				},
				"axis-names": map[int]string{
					1: "Direct",
					2: "Reverse",
				},
			},
		}, {
			Description: "previous period",
			URL:         "/api/v0/console/graph",
			JSONInput: gin.H{
				"start":           time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				"end":             time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				"points":          100,
				"limit":           20,
				"dimensions":      []string{"ExporterName", "InIfProvider"},
				"filter":          "DstCountry = 'FR' AND SrcCountry = 'US'",
				"units":           "l3bps",
				"bidirectional":   false,
				"previous-period": true,
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
					{"Other", "Other"},       // Previous day
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
					{8000, 6000, 4500},
				},
				"min": []int{
					2000,
					100,
					1200,
					1100,
					100,
					100,
					4500,
				},
				"max": []int{
					5000,
					1000,
					1200,
					1100,
					900,
					1900,
					8000,
				},
				"average": []int{
					3333,
					533,
					400,
					366,
					333,
					700,
					6166,
				},
				"95th": []int{
					4000,
					750,
					600,
					550,
					500,
					1000,
					7000,
				},
				"axis": []int{
					1, 1, 1, 1, 1, 1,
					3,
				},
				"axis-names": map[int]string{
					1: "Direct",
					3: "Previous day",
				},
			},
		},
	})
}
