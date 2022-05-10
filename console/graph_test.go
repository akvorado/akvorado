package console

import (
	"bytes"
	"encoding/json"
	"fmt"
	netHTTP "net/http"
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

func TestGraphFilterGroupSQLWhere(t *testing.T) {
	cases := []struct {
		Description string
		Input       graphFilterGroup
		Expected    string
	}{
		{
			Description: "empty group",
			Expected:    "",
		}, {
			Description: "all group",
			Input: graphFilterGroup{
				Operator: graphFilterGroupOperatorAll,
				Rules: []graphFilterRule{
					{
						Column:   graphColumnDstCountry,
						Operator: graphFilterRuleOperatorEqual,
						Value:    "FR",
					}, {
						Column:   graphColumnSrcCountry,
						Operator: graphFilterRuleOperatorEqual,
						Value:    "US",
					},
				},
			},
			Expected: `(DstCountry = 'FR') AND (SrcCountry = 'US')`,
		}, {
			Description: "any group",
			Input: graphFilterGroup{
				Operator: graphFilterGroupOperatorAny,
				Rules: []graphFilterRule{
					{
						Column:   graphColumnDstCountry,
						Operator: graphFilterRuleOperatorEqual,
						Value:    "FR",
					}, {
						Column:   graphColumnSrcCountry,
						Operator: graphFilterRuleOperatorEqual,
						Value:    "US",
					},
				},
			},
			Expected: `(DstCountry = 'FR') OR (SrcCountry = 'US')`,
		}, {
			Description: "nested group",
			Input: graphFilterGroup{
				Operator: graphFilterGroupOperatorAll,
				Rules: []graphFilterRule{
					{
						Column:   graphColumnDstCountry,
						Operator: graphFilterRuleOperatorEqual,
						Value:    "FR",
					},
				},
				Children: []graphFilterGroup{
					{
						Operator: graphFilterGroupOperatorAny,
						Rules: []graphFilterRule{
							{
								Column:   graphColumnSrcCountry,
								Operator: graphFilterRuleOperatorEqual,
								Value:    "US",
							}, {
								Column:   graphColumnSrcCountry,
								Operator: graphFilterRuleOperatorEqual,
								Value:    "IE",
							},
						},
					},
				},
			},
			Expected: `((SrcCountry = 'US') OR (SrcCountry = 'IE')) AND (DstCountry = 'FR')`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			got, _ := tc.Input.toSQLWhere()
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("toSQLWhere (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestGraphFilterRuleSQLWhere(t *testing.T) {
	cases := []struct {
		Description string
		Input       graphFilterRule
		Expected    string
	}{
		{
			Description: "source IP (v4)",
			Input: graphFilterRule{
				Column:   graphColumnSrcAddr,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "192.0.2.11",
			},
			Expected: `SrcAddr = IPv6StringToNum('192.0.2.11')`,
		}, {
			Description: "source IP (v6)",
			Input: graphFilterRule{
				Column:   graphColumnSrcAddr,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "2001:db8::1",
			},
			Expected: `SrcAddr = IPv6StringToNum('2001:db8::1')`,
		}, {
			Description: "source IP (bad)",
			Input: graphFilterRule{
				Column:   graphColumnSrcAddr,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "alfred",
			},
			Expected: "",
		}, {
			Description: "boundary",
			Input: graphFilterRule{
				Column:   graphColumnInIfBoundary,
				Operator: graphFilterRuleOperatorNotEqual,
				Value:    "external",
			},
			Expected: `InIfBoundary != 'external'`,
		}, {
			Description: "boundary (bad)",
			Input: graphFilterRule{
				Column:   graphColumnInIfBoundary,
				Operator: graphFilterRuleOperatorNotEqual,
				Value:    "eternal",
			},
			Expected: "",
		}, {
			Description: "speed",
			Input: graphFilterRule{
				Column:   graphColumnInIfSpeed,
				Operator: graphFilterRuleOperatorLessThan,
				Value:    "1000",
			},
			Expected: `InIfSpeed < 1000`,
		}, {
			Description: "speed (bad)",
			Input: graphFilterRule{
				Column:   graphColumnInIfSpeed,
				Operator: graphFilterRuleOperatorLessThan,
				Value:    "-1000",
			},
			Expected: "",
		}, {
			Description: "source port",
			Input: graphFilterRule{
				Column:   graphColumnSrcPort,
				Operator: graphFilterRuleOperatorLessThan,
				Value:    "1000",
			},
			Expected: `SrcPort < 1000`,
		}, {
			Description: "source port (bad)",
			Input: graphFilterRule{
				Column:   graphColumnSrcPort,
				Operator: graphFilterRuleOperatorLessThan,
				Value:    "10000000",
			},
			Expected: "",
		}, {
			Description: "source AS",
			Input: graphFilterRule{
				Column:   graphColumnSrcAS,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "2906",
			},
			Expected: "SrcAS = 2906",
		}, {
			Description: "source AS (prefixed)",
			Input: graphFilterRule{
				Column:   graphColumnSrcAS,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "AS2906",
			},
			Expected: "SrcAS = 2906",
		}, {
			Description: "source AS (bad)",
			Input: graphFilterRule{
				Column:   graphColumnSrcAS,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "ASMN2906",
			},
			Expected: "",
		}, {
			Description: "EType",
			Input: graphFilterRule{
				Column:   graphColumnEType,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "IPv6",
			},
			Expected: "EType = 34525",
		}, {
			Description: "EType (bad)",
			Input: graphFilterRule{
				Column:   graphColumnEType,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "IPv4+",
			},
			Expected: "",
		}, {
			Description: "Proto (string)",
			Input: graphFilterRule{
				Column:   graphColumnProto,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "TCP",
			},
			Expected: `dictGetOrDefault('protocols', 'name', Proto, '???') = 'TCP'`,
		}, {
			Description: "Proto (int)",
			Input: graphFilterRule{
				Column:   graphColumnProto,
				Operator: graphFilterRuleOperatorEqual,
				Value:    "47",
			},
			Expected: `Proto = 47`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			got, _ := tc.Input.toSQLWhere()
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("toSQLWhere (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestGraphColumnSQLSelect(t *testing.T) {
	cases := []struct {
		Input    graphColumn
		Expected string
	}{
		{
			Input:    graphColumnSrcAddr,
			Expected: `if(SrcAddr IN (SELECT SrcAddr FROM rows), IPv6NumToString(SrcAddr), 'Other')`,
		}, {
			Input:    graphColumnDstAS,
			Expected: `if(DstAS IN (SELECT DstAS FROM rows), concat(toString(DstAS), ': ', dictGetOrDefault('asns', 'name', DstAS, '???')), 'Other')`,
		}, {
			Input:    graphColumnProto,
			Expected: `if(Proto IN (SELECT Proto FROM rows), dictGetOrDefault('protocols', 'name', Proto, '???'), 'Other')`,
		}, {
			Input:    graphColumnEType,
			Expected: `if(EType IN (SELECT EType FROM rows), if(EType = 0x800, 'IPv4', if(EType = 0x86dd, 'IPv6', '???')), 'Other')`,
		}, {
			Input:    graphColumnOutIfSpeed,
			Expected: `if(OutIfSpeed IN (SELECT OutIfSpeed FROM rows), toString(OutIfSpeed), 'Other')`,
		}, {
			Input:    graphColumnExporterName,
			Expected: `if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other')`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Input.String(), func(t *testing.T) {
			got := tc.Input.toSQLSelect()
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("toSQLWhere (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestGraphQuerySQL(t *testing.T) {
	cases := []struct {
		Description string
		Input       graphQuery
		Expected    string
	}{
		{
			Description: "no dimensions, no filters",
			Input: graphQuery{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []graphColumn{},
				Filter:     graphFilterGroup{},
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Bytes*SamplingRate*8/slot) AS bps,
 emptyArrayString() AS dimensions
FROM {table}
WHERE {timefilter}
GROUP BY time, dimensions
ORDER BY time`,
		}, {
			Description: "no dimensions",
			Input: graphQuery{
				Start:      time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:        time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:     100,
				Dimensions: []graphColumn{},
				Filter: graphFilterGroup{
					Operator: graphFilterGroupOperatorAll,
					Rules: []graphFilterRule{
						{
							Column:   graphColumnDstCountry,
							Operator: graphFilterRuleOperatorEqual,
							Value:    "FR",
						}, {
							Column:   graphColumnSrcCountry,
							Operator: graphFilterRuleOperatorEqual,
							Value:    "US",
						},
					},
				},
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Bytes*SamplingRate*8/slot) AS bps,
 emptyArrayString() AS dimensions
FROM {table}
WHERE {timefilter} AND ((DstCountry = 'FR') AND (SrcCountry = 'US'))
GROUP BY time, dimensions
ORDER BY time`,
		}, {
			Description: "no filters",
			Input: graphQuery{
				Start:     time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
				End:       time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
				Points:    100,
				MaxSeries: 20,
				Dimensions: []graphColumn{
					graphColumnExporterName,
					graphColumnInIfProvider,
				},
				Filter: graphFilterGroup{},
			},
			Expected: `
WITH
 intDiv(864, {resolution})*{resolution} AS slot,
 rows AS (SELECT ExporterName, InIfProvider FROM {table} WHERE {timefilter} GROUP BY ExporterName, InIfProvider ORDER BY SUM(Bytes) DESC LIMIT 20)
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS time,
 SUM(Bytes*SamplingRate*8/slot) AS bps,
 [if(ExporterName IN (SELECT ExporterName FROM rows), ExporterName, 'Other'),
  if(InIfProvider IN (SELECT InIfProvider FROM rows), InIfProvider, 'Other')] AS dimensions
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
	helpers.StartStop(t, c)

	base := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	expectedSQL := []struct {
		Time       time.Time `ch:"time"`
		Bps        float64   `ch:"bps"`
		Dimensions []string  `ch:"dimensions"`
	}{
		{base, 1000, []string{"router1", "provider1"}},
		{base, 2000, []string{"router1", "provider2"}},
		{base, 1500, []string{"router1", "Others"}},
		{base, 1200, []string{"router2", "provider2"}},
		{base, 1100, []string{"router2", "provider3"}},
		{base, 1900, []string{"Others", "Others"}},
		{base.Add(time.Minute), 500, []string{"router1", "provider1"}},
		{base.Add(time.Minute), 5000, []string{"router1", "provider2"}},
		{base.Add(time.Minute), 900, []string{"router2", "provider4"}},
		{base.Add(time.Minute), 100, []string{"Others", "Others"}},
		{base.Add(2 * time.Minute), 100, []string{"router1", "provider1"}},
		{base.Add(2 * time.Minute), 3000, []string{"router1", "provider2"}},
		{base.Add(2 * time.Minute), 1500, []string{"router1", "Others"}},
		{base.Add(2 * time.Minute), 100, []string{"router2", "provider4"}},
		{base.Add(2 * time.Minute), 100, []string{"Others", "Others"}},
	}
	expected := gin.H{
		// Sorted by sum of bps
		"rows": [][]string{
			{"router1", "provider2"}, // 10000
			{"router1", "Others"},    // 3000
			{"Others", "Others"},     // 2000
			{"router1", "provider1"}, // 1600
			{"router2", "provider2"}, // 1200
			{"router2", "provider3"}, // 1100
			{"router2", "provider4"}, // 1000
		},
		"t": []string{
			"2009-11-10T23:00:00Z",
			"2009-11-10T23:01:00Z",
			"2009-11-10T23:02:00Z",
		},
		"points": [][]int{
			{2000, 5000, 3000},
			{1500, 0, 1500},
			{1900, 100, 100},
			{1000, 500, 100},
			{1200, 0, 0},
			{1100, 0, 0},
			{0, 900, 100},
		},
		"min": []int{
			2000,
			0,
			100,
			100,
			0,
			0,
			0,
		},
		"max": []int{
			5000,
			1500,
			1900,
			1000,
			1200,
			1100,
			900,
		},
		"average": []int{
			3333,
			1000,
			700,
			533,
			400,
			366,
			333,
		},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), gomock.Any()).
		SetArg(1, expectedSQL).
		Return(nil)

	input := graphQuery{
		Start:     time.Date(2022, 04, 10, 15, 45, 10, 0, time.UTC),
		End:       time.Date(2022, 04, 11, 15, 45, 10, 0, time.UTC),
		Points:    100,
		MaxSeries: 20,
		Dimensions: []graphColumn{
			graphColumnExporterName,
			graphColumnInIfProvider,
		},
		Filter: graphFilterGroup{
			Operator: graphFilterGroupOperatorAll,
			Rules: []graphFilterRule{
				{
					Column:   graphColumnDstCountry,
					Operator: graphFilterRuleOperatorEqual,
					Value:    "FR",
				}, {
					Column:   graphColumnSrcCountry,
					Operator: graphFilterRuleOperatorEqual,
					Value:    "US",
				},
			},
		},
	}
	payload := new(bytes.Buffer)
	err = json.NewEncoder(payload).Encode(input)
	if err != nil {
		t.Fatalf("Encode() error:\n%+v", err)
	}
	resp, err := netHTTP.Post(fmt.Sprintf("http://%s/api/v0/console/graph", h.Address),
		"application/json", payload)
	if err != nil {
		t.Fatalf("POST /api/v0/console/graph:\n%+v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("POST /api/v0/console/graph: got status code %d, not 200", resp.StatusCode)
	}
	gotContentType := resp.Header.Get("Content-Type")
	if gotContentType != "application/json; charset=utf-8" {
		t.Errorf("POST /api/v0/console/graph Content-Type (-got, +want):\n-%s\n+%s",
			gotContentType, "application/json; charset=utf-8")
	}
	decoder := json.NewDecoder(resp.Body)
	var got gin.H
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("POST /api/v0/console/graph error:\n%+v", err)
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("POST /api/v0/console/graph (-got, +want):\n%s", diff)
	}
}
