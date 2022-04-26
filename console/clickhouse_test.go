package console

import (
	"testing"
	"time"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"github.com/golang/mock/gomock"
)

func TestRefreshFlowsTables(t *testing.T) {
	r := reporter.NewMock(t)
	ch, mockConn := clickhousedb.NewMock(t, r)
	h := http.NewMock(t, r)
	c, err := New(r, Configuration{}, Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: ch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

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
		}{{time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC)}})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows_1h0m0s`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 01, 10, 15, 45, 10, 0, time.UTC)}})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows_1m0s`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 04, 20, 15, 45, 10, 0, time.UTC)}})
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `SELECT MIN(TimeReceived) AS t FROM flows_5m0s`).
		Return(nil).
		SetArg(1, []struct {
			T time.Time `ch:"t"`
		}{{time.Date(2022, 02, 10, 15, 45, 10, 0, time.UTC)}})
	if err := c.refreshFlowsTables(); err != nil {
		t.Fatalf("refreshFlowsTables() error:\n%+v", err)
	}

	expected := []flowsTable{
		{"flows", time.Duration(0), time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC)},
		{"flows_1h0m0s", time.Hour, time.Date(2022, 01, 10, 15, 45, 10, 0, time.UTC)},
		{"flows_1m0s", time.Minute, time.Date(2022, 04, 20, 15, 45, 10, 0, time.UTC)},
		{"flows_5m0s", 5 * time.Minute, time.Date(2022, 02, 10, 15, 45, 10, 0, time.UTC)},
	}
	if diff := helpers.Diff(c.flowsTables, expected); diff != "" {
		t.Fatalf("refreshFlowsTables() diff:\n%s", diff)
	}
}

func TestQueryFlowsTables(t *testing.T) {
	cases := []struct {
		Description string
		Tables      []flowsTable
		Query       string
		Start       time.Time
		End         time.Time
		Resolution  time.Duration
		Expected    string
	}{
		{
			Description: "query with source port",
			Query:       "SELECT TimeReceived, SrcPort FROM {table} WHERE {timefilter}",
			Start:       time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:         time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Expected:    "SELECT TimeReceived, SrcPort FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "only flows table available",
			Tables:      []flowsTable{{"flows", 0, time.Date(2022, 03, 10, 15, 45, 10, 0, time.UTC)}},
			Query:       "SELECT 1 FROM {table} WHERE {timefilter}",
			Start:       time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:         time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Expected:    "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "only flows table and out of range request",
			Tables:      []flowsTable{{"flows", 0, time.Date(2022, 04, 10, 22, 45, 10, 0, time.UTC)}},
			Query:       "SELECT 1 FROM {table} WHERE {timefilter}",
			Start:       time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:         time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Expected:    "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "select consolidated table",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 03, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 04, 2, 22, 45, 10, 0, time.UTC)},
			},
			Query:      "SELECT 1 FROM {table} WHERE {timefilter}",
			Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Resolution: 2 * time.Minute,
			Expected:   "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC')",
		}, {
			Description: "select consolidated table out of range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 04, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 04, 10, 17, 45, 10, 0, time.UTC)},
			},
			Query:      "SELECT 1 FROM {table} WHERE {timefilter}",
			Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Resolution: 2 * time.Minute,
			Expected:   "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC')",
		}, {
			Description: "select flows table out of range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 04, 10, 16, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 04, 10, 17, 45, 10, 0, time.UTC)},
			},
			Query:      "SELECT 1 FROM {table} WHERE {timefilter}",
			Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Resolution: 2 * time.Minute,
			Expected:   "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC')",
		}, {
			Description: "select flows table better resolution",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 03, 10, 16, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 03, 10, 17, 45, 10, 0, time.UTC)},
			},
			Query:      "SELECT 1 FROM {table} WHERE {timefilter} // {resolution}",
			Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Resolution: 30 * time.Second,
			Expected:   "SELECT 1 FROM flows WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:10', 'UTC') AND toDateTime('2022-04-11 15:45:10', 'UTC') // 1",
		}, {
			Description: "select consolidated table better resolution",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 03, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 04, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 04, 2, 22, 45, 10, 0, time.UTC)},
			},
			Query:      "SELECT 1 FROM {table} WHERE {timefilter} // {resolution}",
			Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
			End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
			Resolution: 2 * time.Minute,
			Expected:   "SELECT 1 FROM flows_1m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC') // 60",
		}, {
			Description: "select consolidated table better range",
			Tables: []flowsTable{
				{"flows", 0, time.Date(2022, 04, 10, 22, 45, 10, 0, time.UTC)},
				{"flows_5m0s", 5 * time.Minute, time.Date(2022, 04, 2, 22, 45, 10, 0, time.UTC)},
				{"flows_1m0s", time.Minute, time.Date(2022, 04, 10, 22, 45, 10, 0, time.UTC)},
			},
			Query:      "SELECT 1 FROM {table} WHERE {timefilter}",
			Start:      time.Date(2022, 04, 10, 15, 46, 10, 0, time.UTC),
			End:        time.Date(2022, 04, 11, 15, 46, 10, 0, time.UTC),
			Resolution: 2 * time.Minute,
			Expected:   "SELECT 1 FROM flows_5m0s WHERE TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00', 'UTC') AND toDateTime('2022-04-11 15:45:00', 'UTC')",
		},
	}

	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
	h := http.NewMock(t, r)
	c, err := New(r, Configuration{}, Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: ch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			c.flowsTables = tc.Tables
			got := c.queryFlowsTable(tc.Query, tc.Start, tc.End, tc.Resolution)
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("queryFlowsTable(): (-want, +got):\n%s", diff)
			}
		})
	}
}
