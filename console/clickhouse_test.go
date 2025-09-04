// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"
	"time"

	"akvorado/common/helpers"

	"go.uber.org/mock/gomock"
)

func TestRefreshFlowsTables(t *testing.T) {
	c, _, mockConn, _ := NewMock(t, DefaultConfiguration())
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
SELECT name
FROM system.tables
WHERE database=currentDatabase()
AND table LIKE 'flows%'
AND table NOT LIKE '%_local'
AND table != 'flows_raw_errors'
AND (engine LIKE '%MergeTree' OR engine = 'Distributed')
`).
		Return(nil).
		SetArg(1, []struct {
			Name string `ch:"name"`
		}{
			{"flows"},
			{"flows_1h0m0s"},
			{"flows_1m0s"},
			{"flows_5m0s"},
		})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)}})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows_1h0m0s`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 1, 10, 15, 45, 10, 0, time.UTC)}})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows_1m0s`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 4, 20, 15, 45, 10, 0, time.UTC)}})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows_5m0s`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 2, 10, 15, 45, 10, 0, time.UTC)}})
	if err := c.refreshFlowsTables(); err != nil {
		t.Fatalf("refreshFlowsTables() error:\n%+v", err)
	}

	expected := []flowsTable{
		{"flows", time.Duration(0), time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)},
		{"flows_1h0m0s", time.Hour, time.Date(2022, 1, 10, 15, 45, 10, 0, time.UTC)},
		{"flows_1m0s", time.Minute, time.Date(2022, 4, 20, 15, 45, 10, 0, time.UTC)},
		{"flows_5m0s", 5 * time.Minute, time.Date(2022, 2, 10, 15, 45, 10, 0, time.UTC)},
	}
	if diff := helpers.Diff(c.flowsTables, expected); diff != "" {
		t.Fatalf("refreshFlowsTables() diff:\n%s", diff)
	}
}

func TestFinalizeQuery(t *testing.T) {
	cases := []struct {
		Description string
		Tables      []flowsTable
		Query       string
		Context     inputContext
		Expected    string
	}{
		{
			Description: "simple query without additional tables",
			Query:       "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "query with source port",
			Query:       "SELECT TimeReceived, SrcPort FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:             time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:               time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				MainTableRequired: true,
				Points:            86400,
			},
			Expected: "SELECT TimeReceived, SrcPort FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "only flows table available",
			Tables:      []flowsTable{{"flows", 0, time.Date(2022, 3, 10, 15, 45, 10, 0, time.UTC)}},
			Query:       "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "timefilter.Start and timefilter.Stop",
			Tables:      []flowsTable{{"flows", 0, time.Date(2022, 3, 10, 15, 45, 10, 0, time.UTC)}},
			Query:       "SELECT {{ .TimefilterStart }}, {{ .TimefilterEnd }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: "SELECT toDateTime('2022-04-10 15:45:10', 'UTC'), toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "only flows table and out of range request",
			Tables:      []flowsTable{{"flows", 0, time.Date(2022, 4, 10, 22, 45, 10, 0, time.UTC)}},
			Query:       "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "select consolidated table",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 3, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }} // {{ .Interval }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution
			},
			Expected: "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC') // 120",
		}, {
			Description: "select consolidated table out of range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 10, 17, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC')",
		}, {
			Description: "select flows table out of range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 16, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 10, 17, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "use flows table for resolution (control for next case)",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 10, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 3, 10, 10, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }} // {{ .Interval }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 2880, // 30-second resolution
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC') // 30",
		}, {
			Description: "use flows table for resolution and for data",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 10, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 3, 10, 10, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }} // {{ .Interval }}",
			Context: inputContext{
				Start: time.Date(2022, 3, 10, 15, 45, 10, 0, time.UTC),
				End:   time.Date(2022, 3, 11, 15, 45, 10, 0, time.UTC),
				StartForTableSelection: func() *time.Time {
					t := time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)
					return &t
				}(),
				Points: 2880, // 30-second resolution
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-03-10 15:45:10', 'UTC') AND toDateTime('2022-03-11 15:45:10', 'UTC') // 30",
		}, {
			Description: "select flows table with better resolution",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 3, 10, 16, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 3, 10, 17, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }} // {{ .Interval }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 2880,
			},
			Expected: "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC') // 30",
		}, {
			Description: "select consolidated table with better resolution",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 3, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }} // {{ .Interval }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC') // 120",
		}, {
			Description: "select consolidated table with better range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 10, 22, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 46, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 46, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: "SELECT 1 FROM flows_5m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC')",
		}, {
			Description: "select best resolution when equality for oldest data",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 22, 40, 55, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 10, 22, 40, 0, 0, time.UTC)},
				{"flows_1h0m0s", time.Hour, time.Date(2022, 4, 10, 22, 0, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }}",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 46, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 46, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:46:00', 'UTC') AND toDateTime('2022-04-11 15:46:00', 'UTC')",
		}, {
			Description: "query with escaped template",
			Query:       `SELECT TimeReceived, SrcPort WHERE InIfDescription = '{{"{{"}} hello }}'`,
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: `SELECT TimeReceived, SrcPort WHERE InIfDescription = '{{ hello }}'`,
		}, {
			Description: "use of ToStartOfInterval",
			Query:       `{{ call .ToStartOfInterval "TimeReceived" }}`,
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720,
			},
			Expected: `toStartOfInterval(TimeReceived + INTERVAL 50 second, INTERVAL 120 second) - INTERVAL 50 second`,
		}, {
			Description: "Small interval outside main table expiration",
			Query:       "SELECT InIfProvider FROM {{ .Table }}",
			Tables: []flowsTable{
				{"flows", time.Duration(0), time.Date(2022, 11, 6, 12, 0, 0, 0, time.UTC)},
				{"flows_1h0m0s", time.Hour, time.Date(2022, 4, 25, 18, 0, 0, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 11, 14, 12, 0, 0, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 8, 23, 12, 0, 0, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 10, 30, 1, 0, 0, 0, time.UTC),
				End:    time.Date(2022, 10, 30, 12, 0, 0, 0, time.UTC),
				Points: 200,
			},
			Expected: "SELECT InIfProvider FROM flows_5m0s",
		},
	}

	c, _, _, _ := NewMock(t, DefaultConfiguration())
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			c.flowsTables = tc.Tables
			got := c.finalizeTemplateQuery(templateQuery{
				Template: tc.Query,
				Context:  tc.Context,
			})
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("finalizeTemplateQuery(): (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestComputeBestTableAndInterval(t *testing.T) {
	cases := []struct {
		Description string
		Tables      []flowsTable
		Context     inputContext
		Expected    tableIntervalOutput
	}{
		{
			Description: "only flows table",
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "only flows table, require main",
			Context: inputContext{
				Start:             time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:               time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				MainTableRequired: true,
				Points:            86400,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "only flows table available, out of range",
			Tables:      []flowsTable{{"flows", 0, time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)}},
			Context: inputContext{
				Start:  time.Date(2022, 4, 8, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 9, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "consolidated table with better resolution",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 3, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: tableIntervalOutput{Table: "flows_1m0s", Interval: 60},
		}, {
			Description: "consolidated table available, but main required",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 3, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:             time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:               time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points:            720, // 2-minute resolution,
				MainTableRequired: true,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "consolidated table available, but out of range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 3, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 20, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 20, 22, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "consolidated table available, main table required, out of range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 20, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 2, 22, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:             time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:               time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points:            720, // 2-minute resolution,
				MainTableRequired: true,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "empty flows tables list",
			Tables:      []flowsTable{},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 86400,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "target interval smaller than 1 second",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 12, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 10, 15, 45, 20, 0, time.UTC), // 10 seconds with many points
				Points: 100000,
			},
			Expected: tableIntervalOutput{Table: "flows", Interval: 1},
		}, {
			Description: "multiple tables with same resolution, choose oldest data",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 12, 45, 10, 0, time.UTC)},
				{"flows_1m0s_a", time.Minute, time.Date(2022, 4, 9, 12, 45, 10, 0, time.UTC)},
				{"flows_1m0s_b", time.Minute, time.Date(2022, 4, 8, 12, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // 2-minute resolution
			},
			Expected: tableIntervalOutput{Table: "flows_1m0s_b", Interval: 60},
		}, {
			Description: "choose best resolution below target interval",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 8, 12, 45, 10, 0, time.UTC)},
				{"flows_10s", 10 * time.Second, time.Date(2022, 4, 9, 12, 45, 10, 0, time.UTC)},
				{"flows_30s", 30 * time.Second, time.Date(2022, 4, 9, 12, 45, 10, 0, time.UTC)},
				{"flows_2m0s", 2 * time.Minute, time.Date(2022, 4, 9, 12, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 1440, // 1-minute target interval
			},
			Expected: tableIntervalOutput{Table: "flows_30s", Interval: 30},
		}, {
			Description: "all tables out of range, choose table with oldest data",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 15, 12, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 4, 14, 12, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 4, 12, 12, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720,
			},
			Expected: tableIntervalOutput{Table: "flows_5m0s", Interval: 300},
		}, {
			Description: "resolution exactly matches target interval",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 8, 12, 45, 10, 0, time.UTC)},
				{"flows_2m0s", 2 * time.Minute, time.Date(2022, 4, 9, 12, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 720, // Exactly 2-minute interval
			},
			Expected: tableIntervalOutput{Table: "flows_2m0s", Interval: 120},
		}, {
			Description: "sub-second resolution gets clamped to 1 second",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 8, 12, 45, 10, 0, time.UTC)},
				{"flows_100ms", 100 * time.Millisecond, time.Date(2022, 4, 9, 12, 45, 10, 0, time.UTC)},
			},
			Context: inputContext{
				Start:  time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC),
				End:    time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC),
				Points: 8640000, // Very high resolution request
			},
			Expected: tableIntervalOutput{Table: "flows_100ms", Interval: 1}, // Clamped to 1 second
		},
	}

	c, _, _, _ := NewMock(t, DefaultConfiguration())
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			c.flowsTables = tc.Tables
			table, interval, _ := c.computeTableAndInterval(
				tc.Context)
			got := tableIntervalOutput{
				Table:    table,
				Interval: uint64(interval.Seconds()),
			}
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("ComputeBestTableAndInterval(): (-got, +want):\n%s", diff)
			}
		})
	}
}
