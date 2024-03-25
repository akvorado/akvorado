// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package console

import (
	"testing"

	"github.com/benbjohnson/clock"

	"akvorado/common/clickhousedb"
	"akvorado/common/clickhousedb/mocks"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
)

// NewMock instantiates a new authentication component
func NewMock(t *testing.T, config Configuration) (*Component, *httpserver.Component, *mocks.MockConn, *clock.Mock) {
	t.Helper()
	r := reporter.NewMock(t)
	h := httpserver.NewMock(t, r)
	ch, mockConn := clickhousedb.NewMock(t, r)
	mockClock := clock.NewMock()
	c, err := New(r, config, Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: ch,
		Clock:        mockClock,
		Auth:         authentication.NewMock(t, r),
		Database:     database.NewMock(t, r, database.DefaultConfiguration()),
		Schema:       schema.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c, h, mockConn, mockClock
}
