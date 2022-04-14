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

func TestAPILastFlow(t *testing.T) {
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
			URL:         "/api/v0/console/last-flow",
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

func TestAPIExporters(t *testing.T) {
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
			URL:         "/api/v0/console/exporters",
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
