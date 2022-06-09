package console

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"

	"akvorado/common/clickhousedb/mocks"
	"akvorado/common/helpers"
)

func TestWidgetLastFlow(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	ctrl := gomock.NewController(t)
	mockRows := mocks.NewMockRows(ctrl)
	mockConn.EXPECT().Query(gomock.Any(),
		`SELECT * FROM flows WHERE TimeReceived = (SELECT MAX(TimeReceived) FROM flows) LIMIT 1`).
		Return(mockRows, nil)
	mockRows.EXPECT().Next().Return(true)
	mockRows.EXPECT().Close()
	mockRows.EXPECT().Columns().Return([]string{
		"TimeReceived", "SamplingRate",
		"SrcAddr", "SrcCountry",
		"InIfName", "InIfBoundary", "InIfSpeed",
	}).AnyTimes()

	colTimeReceived := mocks.NewMockColumnType(ctrl)
	colSamplingRate := mocks.NewMockColumnType(ctrl)
	colSrcAddr := mocks.NewMockColumnType(ctrl)
	colSrcCountry := mocks.NewMockColumnType(ctrl)
	colInIfName := mocks.NewMockColumnType(ctrl)
	colInIfBoundary := mocks.NewMockColumnType(ctrl)
	colInIfSpeed := mocks.NewMockColumnType(ctrl)
	colTimeReceived.EXPECT().ScanType().Return(reflect.TypeOf(time.Time{}))
	colSamplingRate.EXPECT().ScanType().Return(reflect.TypeOf(uint64(0)))
	colSrcAddr.EXPECT().ScanType().Return(reflect.TypeOf(net.IP{}))
	colSrcCountry.EXPECT().ScanType().Return(reflect.TypeOf(""))
	colInIfName.EXPECT().ScanType().Return(reflect.TypeOf(""))
	colInIfBoundary.EXPECT().ScanType().Return(reflect.TypeOf(""))
	colInIfSpeed.EXPECT().ScanType().Return(reflect.TypeOf(uint32(0)))
	mockRows.EXPECT().ColumnTypes().Return([]driver.ColumnType{
		colTimeReceived,
		colSamplingRate,
		colSrcAddr,
		colSrcCountry,
		colInIfName,
		colInIfBoundary,
		colInIfSpeed,
	}).AnyTimes()

	mockRows.EXPECT().Scan(gomock.Any()).
		DoAndReturn(func(args ...interface{}) interface{} {
			arg0 := args[0].(*time.Time)
			*arg0 = time.Date(2022, 4, 4, 8, 36, 11, 10, time.UTC)
			arg1 := args[1].(*uint64)
			*arg1 = uint64(10000)
			arg2 := args[2].(*net.IP)
			*arg2 = net.ParseIP("2001:db8::22")
			arg3 := args[3].(*string)
			*arg3 = "FR"
			arg4 := args[4].(*string)
			*arg4 = "Hu0/0/1/10"
			arg5 := args[5].(*string)
			*arg5 = "external"
			arg6 := args[6].(*uint32)
			*arg6 = uint32(100000)
			return nil
		})

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/widget/flow-last",
			JSONOutput: gin.H{
				"InIfBoundary": "external",
				"InIfName":     "Hu0/0/1/10",
				"InIfSpeed":    100000,
				"SamplingRate": 10000,
				"SrcAddr":      "2001:db8::22",
				"SrcCountry":   "FR",
				"TimeReceived": "2022-04-04T08:36:11.00000001Z",
			},
		},
	})
}

func TestFlowRate(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	ctrl := gomock.NewController(t)
	mockRow := mocks.NewMockRow(ctrl)
	mockRow.EXPECT().Err().Return(nil)
	mockRow.EXPECT().Scan(gomock.Any()).SetArg(0, float64(100.1)).Return(nil)
	mockConn.EXPECT().
		QueryRow(gomock.Any(),
			`SELECT COUNT(*)/300 AS rate FROM flows WHERE TimeReceived > date_sub(minute, 5, now())`).
		Return(mockRow)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/widget/flow-rate",
			JSONOutput: gin.H{
				"period": "second",
				"rate":   100.1,
			},
		},
	})
}

func TestWidgetExporters(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	expected := []struct {
		ExporterName string
	}{
		{"exporter1"},
		{"exporter2"},
		{"exporter3"},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(),
			`SELECT ExporterName FROM exporters GROUP BY ExporterName ORDER BY ExporterName`).
		SetArg(1, expected).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/widget/exporters",
			JSONOutput: gin.H{
				"exporters": []string{
					"exporter1",
					"exporter2",
					"exporter3",
				},
			},
		},
	})
}

func TestWidgetTop(t *testing.T) {
	_, h, mockConn, _ := NewMock(t, DefaultConfiguration())

	gomock.InOrder(
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"TCP/443", float64(51)},
				{"UDP/443", float64(20)},
				{"TCP/80", float64(18)},
			}),
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"TCP", float64(75)},
				{"UDP", float64(24)},
				{"ESP", float64(1)},
			}),
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"exporter1", float64(20)},
				{"exporter3", float64(10)},
				{"exporter5", float64(3)},
			}),
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"2906: Netflix", float64(12)},
				{"36040: Youtube", float64(10)},
				{"20940: Akamai", float64(9)},
			}),
	)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/widget/top/src-port",
			JSONOutput: gin.H{
				"top": []gin.H{
					{"name": "TCP/443", "percent": 51},
					{"name": "UDP/443", "percent": 20},
					{"name": "TCP/80", "percent": 18}}},
		}, {
			URL: "/api/v0/console/widget/top/protocol",
			JSONOutput: gin.H{
				"top": []gin.H{
					{"name": "TCP", "percent": 75},
					{"name": "UDP", "percent": 24},
					{"name": "ESP", "percent": 1}}},
		}, {
			URL: "/api/v0/console/widget/top/exporter",
			JSONOutput: gin.H{
				"top": []gin.H{
					{"name": "exporter1", "percent": 20},
					{"name": "exporter3", "percent": 10},
					{"name": "exporter5", "percent": 3}}},
		}, {
			URL: "/api/v0/console/widget/top/src-as",
			JSONOutput: gin.H{
				"top": []gin.H{
					{"name": "2906: Netflix", "percent": 12},
					{"name": "36040: Youtube", "percent": 10},
					{"name": "20940: Akamai", "percent": 9}}},
		},
	})
}

func TestWidgetGraph(t *testing.T) {
	_, h, mockConn, mockClock := NewMock(t, DefaultConfiguration())

	base := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	mockClock.Set(base.Add(24 * time.Hour))
	expected := []struct {
		Time time.Time `json:"t"`
		Gbps float64   `json:"gbps"`
	}{
		{base, 25.3},
		{base.Add(time.Minute), 27.8},
		{base.Add(2 * time.Minute), 26.4},
		{base.Add(3 * time.Minute), 29.2},
		{base.Add(4 * time.Minute), 21.3},
		{base.Add(5 * time.Minute), 24.7},
	}
	mockConn.EXPECT().
		Select(gomock.Any(), gomock.Any(), `
WITH
 intDiv(864, 1)*1 AS slot
SELECT
 toStartOfInterval(TimeReceived, INTERVAL slot second) AS Time,
 SUM(Bytes*SamplingRate*8/slot)/1000/1000/1000 AS Gbps
FROM flows
WHERE TimeReceived BETWEEN toDateTime('2009-11-10 23:00:00', 'UTC') AND toDateTime('2009-11-11 23:00:00', 'UTC')
AND InIfBoundary = 'external'
GROUP BY Time
ORDER BY Time`).
		SetArg(1, expected).
		Return(nil)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/widget/graph?points=100",
			JSONOutput: gin.H{
				"data": []gin.H{
					{"t": "2009-11-10T23:00:00Z", "gbps": 25.3},
					{"t": "2009-11-10T23:01:00Z", "gbps": 27.8},
					{"t": "2009-11-10T23:02:00Z", "gbps": 26.4},
					{"t": "2009-11-10T23:03:00Z", "gbps": 29.2},
					{"t": "2009-11-10T23:04:00Z", "gbps": 21.3},
					{"t": "2009-11-10T23:05:00Z", "gbps": 24.7}},
			},
		},
	})

}
