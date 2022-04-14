package console

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/golang/mock/gomock"

	"akvorado/common/clickhousedb"
	"akvorado/common/clickhousedb/mocks"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestWidgetLastFlow(t *testing.T) {
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
			URL:         "/api/v0/console/widget/flow-last",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{`,
				`    "InIfBoundary": "external",`,
				`    "InIfName": "Hu0/0/1/10",`,
				`    "InIfSpeed": 100000,`,
				`    "SamplingRate": 10000,`,
				`    "SrcAddr": "2001:db8::22",`,
				`    "SrcCountry": "FR",`,
				`    "TimeReceived": "2022-04-04T08:36:11.00000001Z"`,
				`}`,
			},
		},
	})
}

func TestFlowRate(t *testing.T) {
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
			URL:         "/api/v0/console/widget/flow-rate",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{`,
				`    "period": "second",`,
				`    "rate": 100.1`,
				`}`,
			},
		},
	})
}

func TestWidgetExporters(t *testing.T) {
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
			URL:         "/api/v0/console/widget/exporters",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{`,
				`    "exporters": [`,
				`        "exporter1",`,
				`        "exporter2",`,
				`        "exporter3"`,
				`    ]`,
				`}`,
			},
		},
	})

}

func TestWidgetTop(t *testing.T) {
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

	gomock.InOrder(
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"TCP/443", uint8(51)},
				{"UDP/443", uint8(20)},
				{"TCP/80", uint8(18)},
			}),
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"TCP", uint8(75)},
				{"UDP", uint8(24)},
				{"ESP", uint8(1)},
			}),
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"exporter1", uint8(20)},
				{"exporter3", uint8(10)},
				{"exporter5", uint8(3)},
			}),
		mockConn.EXPECT().
			Select(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).
			SetArg(1, []topResult{
				{"2906: Netflix", uint8(12)},
				{"36040: Youtube", uint8(10)},
				{"20940: Akamai", uint8(9)},
			}),
	)

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/console/widget/top/src-port",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{"top":[{"name":"TCP/443","percent":51},{"name":"UDP/443","percent":20},{"name":"TCP/80","percent":18}]}`,
			},
		}, {
			URL:         "/api/v0/console/widget/top/protocol",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{"top":[{"name":"TCP","percent":75},{"name":"UDP","percent":24},{"name":"ESP","percent":1}]}`,
			},
		}, {
			URL:         "/api/v0/console/widget/top/exporter",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{"top":[{"name":"exporter1","percent":20},{"name":"exporter3","percent":10},{"name":"exporter5","percent":3}]}`,
			},
		}, {
			URL:         "/api/v0/console/widget/top/src-as",
			ContentType: "application/json; charset=utf-8",
			FirstLines: []string{
				`{"top":[{"name":"2906: Netflix","percent":12},{"name":"36040: Youtube","percent":10},{"name":"20940: Akamai","percent":9}]}`,
			},
		},
	})
}
