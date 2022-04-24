//go:build !release

package clickhousedb

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"akvorado/common/clickhousedb/mocks"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// SetupClickHouse configures a client to use for testing.
func SetupClickHouse(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	chServer := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse", "localhost"}, "9000")
	config := DefaultConfiguration()
	config.Servers = []string{chServer}
	config.DialTimeout = 100 * time.Millisecond
	c, err := New(r, config, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
}

// NewMock creates a new component using a mock driver. It returns
// both the component and the mock driver.
func NewMock(t *testing.T, r *reporter.Reporter) (*Component, *mocks.MockConn) {
	t.Helper()
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	ctrl := gomock.NewController(t)
	mock := mocks.NewMockConn(ctrl)
	c.Conn = mock

	mock.EXPECT().
		Close().
		Return(nil)

	helpers.StartStop(t, c)
	return c, mock
}
