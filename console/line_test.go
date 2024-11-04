// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	"akvorado/console/query"
)

func TestGraphLineInputReverseDirection(t *testing.T) {
	input := graphLineHandlerInput{
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
		Points: 100,
	}
	original1 := fmt.Sprintf("%+v", input)
	expected := graphLineHandlerInput{
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
		Points: 100,
	}
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

func TestGraphPreviousPeriod(t *testing.T) {
	const longForm = "Jan 2, 2006 at 15:04"
	cases := []struct {
		Pos           helpers.Pos
		Start         string
		End           string
		ExpectedStart string
		ExpectedEnd   string
	}{
		{
			helpers.Mark(),
			"Jan 2, 2020 at 15:04", "Jan 2, 2020 at 16:04",
			"Jan 2, 2020 at 14:04", "Jan 2, 2020 at 15:04",
		}, {
			helpers.Mark(),
			"Jan 2, 2020 at 15:04", "Jan 2, 2020 at 16:34",
			"Jan 2, 2020 at 14:04", "Jan 2, 2020 at 15:34",
		}, {
			helpers.Mark(),
			"Jan 2, 2020 at 15:04", "Jan 2, 2020 at 17:34",
			"Jan 1, 2020 at 15:04", "Jan 1, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Jan 2, 2020 at 15:04", "Jan 3, 2020 at 17:34",
			"Jan 1, 2020 at 15:04", "Jan 2, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Jan 10, 2020 at 15:04", "Jan 13, 2020 at 17:34",
			"Jan 3, 2020 at 15:04", "Jan 6, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Jan 10, 2020 at 15:04", "Jan 15, 2020 at 17:34",
			"Jan 3, 2020 at 15:04", "Jan 8, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Jan 10, 2020 at 15:04", "Jan 20, 2020 at 17:34",
			"Jan 3, 2020 at 15:04", "Jan 13, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Feb 10, 2020 at 15:04", "Feb 25, 2020 at 17:34",
			"Jan 13, 2020 at 15:04", "Jan 28, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Feb 10, 2020 at 15:04", "Mar 25, 2020 at 17:34",
			"Jan 13, 2020 at 15:04", "Feb 26, 2020 at 17:34",
		}, {
			helpers.Mark(),
			"Feb 10, 2020 at 15:04", "Jul 25, 2020 at 17:34",
			"Feb 10, 2019 at 15:04", "Jul 25, 2019 at 17:34",
		}, {
			helpers.Mark(),
			"Feb 10, 2019 at 15:04", "Jul 25, 2020 at 17:34",
			"Feb 10, 2018 at 15:04", "Jul 25, 2019 at 17:34",
		},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s to %s", tc.Start, tc.End), func(t *testing.T) {
			start, err := time.Parse(longForm, tc.Start)
			if err != nil {
				t.Fatalf("%stime.Parse(%q) error:\n%+v", tc.Pos, tc.Start, err)
			}
			end, err := time.Parse(longForm, tc.End)
			if err != nil {
				t.Fatalf("%stime.Parse(%q) error:\n%+v", tc.Pos, tc.End, err)
			}
			expectedStart, err := time.Parse(longForm, tc.ExpectedStart)
			if err != nil {
				t.Fatalf("%stime.Parse(%q) error:\n%+v", tc.Pos, tc.ExpectedStart, err)
			}
			expectedEnd, err := time.Parse(longForm, tc.ExpectedEnd)
			if err != nil {
				t.Fatalf("%stime.Parse(%q) error:\n%+v", tc.Pos, tc.ExpectedEnd, err)
			}
			input := graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					schema: schema.NewMock(t),
					Start:  start,
					End:    end,
					Dimensions: query.Columns{
						query.NewColumn("ExporterAddress"),
						query.NewColumn("ExporterName"),
					},
				},
			}
			query.Columns(input.Dimensions).Validate(input.schema)
			got := input.previousPeriod()
			expected := graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      expectedStart,
					End:        expectedEnd,
					Dimensions: []query.Column{},
				},
			}
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("%spreviousPeriod() (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}

func TestGraphQuerySQL(t *testing.T) {
	cases := []struct {
		Description string
		Pos         helpers.Pos
		Input       graphLineHandlerInput
		Expected    string
	}{
		{
			Description: "no dimensions, no filters, bps",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.Filter{},
					Units:      "l3bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}`,
		}, {
			Description: "no dimensions, no filters, l2 bps",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.Filter{},
					Units:      "l2bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l2bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}
`,
		}, {
			Description: "no dimensions, no filters, pps",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.Filter{},
					Units:      "pps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"pps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}`,
		}, {
			Description: "truncated source address",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:          time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:            time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions:     []query.Column{query.NewColumn("SrcAddr")},
					Filter:         query.NewFilter("SrcAddr << 1.0.0.0/8"),
					TruncateAddrV4: 24,
					TruncateAddrV6: 48,
					Units:          "l3bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","main-table-required":true,"points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * REPLACE (tupleElement(IPv6CIDRToRange(SrcAddr, if(tupleElement(IPv6CIDRToRange(SrcAddr, 96), 1) = toIPv6('::ffff:0.0.0.0'), 120, 48)), 1) AS SrcAddr) FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 rows AS (SELECT SrcAddr FROM source WHERE {{ .Timefilter }} AND (SrcAddr BETWEEN toIPv6('::ffff:1.0.0.0') AND toIPv6('::ffff:1.255.255.255')) GROUP BY SrcAddr ORDER BY {{ .Units }} DESC LIMIT 0)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((SrcAddr) IN rows, [replaceRegexpOne(IPv6NumToString(SrcAddr), '^::ffff:', '')], ['Other']) AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (SrcAddr BETWEEN toIPv6('::ffff:1.0.0.0') AND toIPv6('::ffff:1.255.255.255'))
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS ['Other']))
{{ end }}`,
		}, {
			Description: "no dimensions",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.NewFilter("DstCountry = 'FR' AND SrcCountry = 'US'"),
					Units:      "l3bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (DstCountry = 'FR' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}`,
		}, {
			Description: "no dimensions, escaped filter",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.NewFilter("InIfDescription = '{{ hello }}' AND SrcCountry = 'US'"),
					Units:      "l3bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (InIfDescription = '{{"{{"}} hello }}' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}`,
		}, {
			Description: "no dimensions, reverse direction",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.NewFilter("DstCountry = 'FR' AND SrcCountry = 'US'"),
					Units:      "l3bps",
				},
				Points:        100,
				Bidirectional: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (DstCountry = 'FR' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
SELECT 2 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (SrcCountry = 'FR' AND DstCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}`,
		}, {
			Description: "no dimensions, reverse direction, inl2%",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:      time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:        time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Dimensions: []query.Column{},
					Filter:     query.NewFilter("DstCountry = 'FR' AND SrcCountry = 'US'"),
					Units:      "inl2%",
				},
				Points:        100,
				Bidirectional: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"inl2%","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (DstCountry = 'FR' AND SrcCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"outl2%","aggregator":"SUM"}@@ }}
SELECT 2 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }} AND (SrcCountry = 'FR' AND DstCountry = 'US')
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
{{ end }}`,
		}, {
			Description: "no filters",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Limit: 20,
					Dimensions: []query.Column{
						query.NewColumn("ExporterName"),
						query.NewColumn("InIfProvider"),
					},
					Filter: query.Filter{},
					Units:  "l3bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 rows AS (SELECT ExporterName, InIfProvider FROM source WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider ORDER BY {{ .Units }} DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS ['Other', 'Other']))
{{ end }}`,
		}, {
			Description: "no filters, limitType by max",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start:     time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:       time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Limit:     20,
					LimitType: "Max",
					Dimensions: []query.Column{
						query.NewColumn("ExporterName"),
						query.NewColumn("InIfProvider"),
					},
					Filter: query.Filter{},
					Units:  "l3bps",
				},
				Points: 100,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"MAX"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 rows AS (SELECT ExporterName, InIfProvider FROM ( SELECT ExporterName, InIfProvider, {{ .Units }} AS max_at_time FROM source WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider, {{ .Timefilter }} ) GROUP BY ExporterName, InIfProvider ORDER BY MAX(max_at_time) DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS ['Other', 'Other']))
{{ end }}`,
		}, {
			Description: "no filters, reverse",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Limit: 20,
					Dimensions: []query.Column{
						query.NewColumn("ExporterName"),
						query.NewColumn("InIfProvider"),
					},
					Filter: query.Filter{},
					Units:  "l3bps",
				},
				Points:        100,
				Bidirectional: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 rows AS (SELECT ExporterName, InIfProvider FROM source WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider ORDER BY {{ .Units }} DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS ['Other', 'Other']))
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
SELECT 2 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, OutIfProvider) IN rows, [ExporterName, OutIfProvider], ['Other', 'Other']) AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS ['Other', 'Other']))
{{ end }}`,
		}, {
			Description: "no filters, previous period",
			Pos:         helpers.Mark(),
			Input: graphLineHandlerInput{
				graphCommonHandlerInput: graphCommonHandlerInput{
					Start: time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					End:   time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					Limit: 20,
					Dimensions: []query.Column{
						query.NewColumn("ExporterName"),
						query.NewColumn("InIfProvider"),
					},
					Filter: query.Filter{},
					Units:  "l3bps",
				},
				Points:         100,
				PreviousPeriod: true,
			},
			Expected: `
{{ with context @@{"start":"2022-04-10T15:45:10Z","end":"2022-04-11T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
WITH
 source AS (SELECT * FROM {{ .Table }} SETTINGS asterisk_include_alias_columns = 1),
 rows AS (SELECT ExporterName, InIfProvider FROM source WHERE {{ .Timefilter }} GROUP BY ExporterName, InIfProvider ORDER BY {{ .Units }} DESC LIMIT 20)
SELECT 1 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 if((ExporterName, InIfProvider) IN rows, [ExporterName, InIfProvider], ['Other', 'Other']) AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }}
 TO {{ .TimefilterEnd }} + INTERVAL 1 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS ['Other', 'Other']))
{{ end }}
UNION ALL
{{ with context @@{"start":"2022-04-09T15:45:10Z","end":"2022-04-10T15:45:10Z","start-for-interval":"2022-04-10T15:45:10Z","points":100,"units":"l3bps","aggregator":"SUM"}@@ }}
SELECT 3 AS axis, * FROM (
SELECT
 {{ call .ToStartOfInterval "TimeReceived" }} + INTERVAL 86400 second AS time,
 {{ .Units }}/{{ .Interval }} AS xps,
 emptyArrayString() AS dimensions
FROM source
WHERE {{ .Timefilter }}
GROUP BY time, dimensions
ORDER BY time WITH FILL
 FROM {{ .TimefilterStart }} + INTERVAL 86400 second
 TO {{ .TimefilterEnd }} + INTERVAL 1 second + INTERVAL 86400 second
 STEP {{ .Interval }}
 INTERPOLATE (dimensions AS emptyArrayString()))
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
			got := tc.Input.toSQL()
			if diff := helpers.Diff(strings.Split(strings.TrimSpace(got), "\n"),
				strings.Split(strings.TrimSpace(tc.Expected), "\n")); diff != "" {
				t.Errorf("%stoSQL (-got, +want):\n%s", tc.Pos, diff)
			}
		})
	}
}

func TestGraphLineHandler(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())
	base := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	t.Run("sort by avg", func(t *testing.T) {
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

		helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "single direction",
				URL:         "/api/v0/console/graph/line",
				JSONInput: gin.H{
					"start":         time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					"end":           time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					"points":        100,
					"limit":         20,
					"limitType":     "Avg",
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
				URL:         "/api/v0/console/graph/line",
				JSONInput: gin.H{
					"start":         time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					"end":           time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					"points":        100,
					"limit":         20,
					"limitType":     "Avg",
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
				URL:         "/api/v0/console/graph/line",
				JSONInput: gin.H{
					"start":           time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					"end":             time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					"points":          100,
					"limit":           20,
					"limitType":       "Avg",
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
	})

	t.Run("sort by max", func(t *testing.T) {
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

		helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "single direction",
				URL:         "/api/v0/console/graph/line",
				JSONInput: gin.H{
					"start":         time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					"end":           time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					"points":        100,
					"limit":         20,
					"limitType":     "Max",
					"dimensions":    []string{"ExporterName", "InIfProvider"},
					"filter":        "DstCountry = 'FR' AND SrcCountry = 'US'",
					"units":         "l3bps",
					"bidirectional": false,
				},
				JSONOutput: gin.H{
					// Sorted by max of bps
					"rows": [][]string{
						{"router1", "provider2"},
						{"router2", "provider2"},
						{"router2", "provider3"},
						{"router1", "provider1"},
						{"router2", "provider4"},
						{"Other", "Other"},
					},
					"t": []string{
						"2009-11-10T23:00:00Z",
						"2009-11-10T23:01:00Z",
						"2009-11-10T23:02:00Z",
					},
					"points": [][]int{
						{2000, 5000, 3000},
						{1200, 0, 0},
						{1100, 0, 0},
						{1000, 500, 100},
						{0, 900, 100},
						{1900, 100, 100},
					},
					"min": []int{
						2000,
						1200,
						1100,
						100,
						100,
						100,
					},
					"max": []int{
						5000,
						1200,
						1100,
						1000,
						900,
						1900,
					},
					"average": []int{
						3333,
						400,
						366,
						533,
						333,
						700,
					},
					"95th": []int{
						4000,
						600,
						550,
						750,
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
				URL:         "/api/v0/console/graph/line",
				JSONInput: gin.H{
					"start":         time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					"end":           time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					"points":        100,
					"limit":         20,
					"limitType":     "Max",
					"dimensions":    []string{"ExporterName", "InIfProvider"},
					"filter":        "DstCountry = 'FR' AND SrcCountry = 'US'",
					"units":         "l3bps",
					"bidirectional": true,
				},
				JSONOutput: gin.H{
					// Sorted by sum of bps
					"rows": [][]string{
						{"router1", "provider2"}, // 10000
						{"router2", "provider2"}, // 1200
						{"router2", "provider3"}, // 1100
						{"router1", "provider1"}, // 1600
						{"router2", "provider4"}, // 1000
						{"Other", "Other"},       // 2100

						{"router1", "provider2"}, // 1000
						{"router2", "provider2"}, // 120
						{"router2", "provider3"}, // 110
						{"router1", "provider1"}, // 160
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
						{1200, 0, 0},
						{1100, 0, 0},
						{1000, 500, 100},
						{0, 900, 100},
						{1900, 100, 100},

						{200, 500, 300},
						{120, 0, 0},
						{110, 0, 0},
						{100, 50, 10},
						{0, 90, 10},
						{190, 10, 10},
					},
					"min": []int{
						2000,
						1200,
						1100,
						100,
						100,
						100,

						200,
						120,
						110,
						10,
						10,
						10,
					},
					"max": []int{
						5000,
						1200,
						1100,
						1000,
						900,
						1900,

						500,
						120,
						110,
						100,
						90,
						190,
					},
					"average": []int{
						3333,
						400,
						366,
						533,
						333,
						700,

						333,
						40,
						36,
						53,
						33,
						70,
					},
					"95th": []int{
						4000,
						600,
						550,
						750,
						500,
						1000,

						400,
						60,
						55,
						75,
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
				URL:         "/api/v0/console/graph/line",
				JSONInput: gin.H{
					"start":           time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
					"end":             time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
					"points":          100,
					"limit":           20,
					"limitType":       "Max",
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
						{"router2", "provider2"}, // 1200
						{"router2", "provider3"}, // 1100
						{"router1", "provider1"}, // 1600
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
						{1200, 0, 0},
						{1100, 0, 0},
						{1000, 500, 100},
						{0, 900, 100},
						{1900, 100, 100},
						{8000, 6000, 4500},
					},
					"min": []int{
						2000,
						1200,
						1100,
						100,
						100,
						100,
						4500,
					},
					"max": []int{
						5000,
						1200,
						1100,
						1000,
						900,
						1900,
						8000,
					},
					"average": []int{
						3333,
						400,
						366,
						533,
						333,
						700,
						6166,
					},
					"95th": []int{
						4000,
						600,
						550,
						750,
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
	})
}

func TestGetTableInterval(t *testing.T) {
	_, h, _, mockClock := NewMock(t, DefaultConfiguration())
	mockClock.Set(time.Date(2022, 4, 12, 15, 45, 10, 0, time.UTC))
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "simple query",
			URL:         "/api/v0/console/graph/table-interval",
			JSONInput: gin.H{
				"start":  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				"end":    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				"points": 300,
			},
			JSONOutput: gin.H{
				"table":    "flows",
				"interval": 1,
			},
		}, {
			Description: "too many points",
			URL:         "/api/v0/console/graph/table-interval",
			JSONInput: gin.H{
				"start":  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				"end":    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				"points": 86400,
			},
			StatusCode: 400,
			JSONOutput: gin.H{
				"message": `Key: 'tableIntervalInput.Points' Error:Field validation for 'Points' failed on the 'max' tag`,
			},
		},
	})
}
