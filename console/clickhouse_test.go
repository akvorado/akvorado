// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
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
AND engine LIKE '%MergeTree'
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
			Description: "use flows table for resolution (but flows_1m0s for data)",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 4, 10, 10, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 3, 10, 10, 45, 10, 0, time.UTC)},
			},
			Query: "SELECT 1 FROM {{ .Table }} WHERE {{ .Timefilter }} // {{ .Interval }}",
			Context: inputContext{
				Start: time.Date(2022, 3, 10, 15, 45, 10, 0, time.UTC),
				End:   time.Date(2022, 3, 11, 15, 45, 10, 0, time.UTC),
				StartForInterval: func() *time.Time {
					t := time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)
					return &t
				}(),
				Points: 2880, // 30-second resolution
			},
			Expected: "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-03-10 15:45:10', 'UTC') AND toDateTime('2022-03-11 15:45:10', 'UTC') // 30",
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
			got := c.finalizeQuery(
				fmt.Sprintf(`{{ with %s }}%s{{ end }}`, templateContext(tc.Context), tc.Query))
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("finalizeQuery(): (-got, +want):\n%s", diff)
			}
		})
	}
}
