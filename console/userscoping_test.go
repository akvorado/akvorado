// SPDX-FileCopyrightText: 2026 Gabriel ROUSSEAU (bolemo)
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/benbjohnson/clock"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
)

// newTestComponent builds a console component around the provided ClickHouse.
func newTestComponent(t *testing.T, r *reporter.Reporter, ch *clickhousedb.Component, h *httpserver.Component, auth *authentication.Component, setting string) *Component {
	t.Helper()
	config := DefaultConfiguration()
	config.ClickHouseUserSetting = setting
	c, err := New(r, config, Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: ch,
		Clock:        clock.NewMock(),
		Auth:         auth,
		Database:     database.NewMock(t, r, database.DefaultConfiguration()),
		Schema:       schema.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}

// TestClickHouseUserSettingName checks an unusable setting name is rejected on
// startup instead of breaking every query.
func TestClickHouseUserSettingName(t *testing.T) {
	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
	cases := []struct {
		Description string
		Setting     string
		Invalid     bool
	}{
		{Description: "disabled"},
		{Description: "usual name", Setting: "SQL_akvorado_user"},
		{Description: "leading underscore", Setting: "_user"},
		{Description: "leading digit", Setting: "1user", Invalid: true},
		{Description: "dash", Setting: "SQL-user", Invalid: true},
		{Description: "space", Setting: "SQL_akvorado user", Invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			config := DefaultConfiguration()
			config.ClickHouseUserSetting = tc.Setting
			_, err := New(r, config, Dependencies{
				Daemon:       daemon.NewMock(t),
				HTTP:         httpserver.NewMock(t, r),
				ClickHouseDB: ch,
				Clock:        clock.NewMock(),
				Auth:         authentication.NewMock(t, r),
				Database:     database.NewMock(t, r, database.DefaultConfiguration()),
				Schema:       schema.NewMock(t),
			})
			if tc.Invalid && err == nil {
				t.Errorf("New() with setting %q did not return an error", tc.Setting)
			}
			if !tc.Invalid && err != nil {
				t.Errorf("New() error:\n%+v", err)
			}
		})
	}
}

// TestUserSettings checks the setting carrying the login is only built when the
// feature is enabled, and that it is flagged as a custom one.
func TestUserSettings(t *testing.T) {
	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
	h := httpserver.NewMock(t, r)
	auth := authentication.NewMock(t, r)

	t.Run("enabled", func(t *testing.T) {
		c := newTestComponent(t, r, ch, h, auth, "SQL_akvorado_user")
		got := c.userSettings("alfred")
		expected := clickhouse.Settings{
			"SQL_akvorado_user": clickhouse.CustomSetting{Value: "alfred"},
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("userSettings() (-got, +want):\n%s", diff)
		}
	})
	t.Run("disabled", func(t *testing.T) {
		c := newTestComponent(t, r, ch, h, auth, "")
		if got := c.userSettings("alfred"); got != nil {
			t.Fatalf("userSettings() got %v, expected nil", got)
		}
	})
}

// TestUserScoping checks a row policy relying on the setting really keeps a
// user from seeing the rows of another one. This is the test proving the
// feature: it needs a real server, as the custom flag the setting is sent with
// is only checked there.
func TestUserScoping(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r, false)
	h := httpserver.NewMock(t, r)
	auth := authentication.NewMock(t, r)
	c := newTestComponent(t, r, chComponent, h, auth, "SQL_akvorado_user")

	// A stand-in for a flows table, scoped with a row policy on the tenant
	// attribute, the way an operator would do it.
	db := chComponent.DatabaseName()
	policy := fmt.Sprintf("scope_%s", db)
	for _, query := range []string{
		fmt.Sprintf(`CREATE TABLE %s.scoped (Tenant String, Bytes UInt64)
                     ENGINE = MergeTree ORDER BY Tenant`, db),
		fmt.Sprintf(`INSERT INTO %s.scoped VALUES
                     ('alfred', 10), ('alfred', 20), ('bernard', 100)`, db),
		fmt.Sprintf(`CREATE ROW POLICY %s ON %s.scoped FOR SELECT
                     USING Tenant = getSetting('SQL_akvorado_user') TO default`,
			policy, db),
	} {
		if err := chComponent.Exec(t.Context(), query); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", query, err)
		}
	}
	t.Cleanup(func() {
		// t.Context() is already cancelled at this point. The policy outlives
		// the database, so it has to be dropped explicitly.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		chComponent.Exec(ctx, fmt.Sprintf("DROP ROW POLICY %s ON %s.scoped", policy, db))
	})

	// A handler querying ClickHouse the way the real ones do, from the context
	// of the request.
	h.APIRouter.GET("/api/v0/console/scoped",
		func(w http.ResponseWriter, req *http.Request) {
			results := []struct {
				Tenant string `ch:"Tenant"`
				Bytes  uint64 `ch:"Bytes"`
			}{}
			err := chComponent.Select(req.Context(), &results,
				fmt.Sprintf(`SELECT Tenant, SUM(Bytes) AS Bytes FROM %s.scoped
                             GROUP BY Tenant ORDER BY Tenant`, db))
			if err != nil {
				httpserver.WriteJSON(w, http.StatusInternalServerError,
					helpers.M{"message": err.Error()})
				return
			}
			rows := []helpers.M{}
			for _, result := range results {
				rows = append(rows, helpers.M{"tenant": result.Tenant, "bytes": result.Bytes})
			}
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{"rows": rows})
		},
		auth.UserAuthentication(), c.userScoping())

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "a user only sees their own rows",
			URL:         "/api/v0/console/scoped",
			Header:      http.Header{"Remote-User": []string{"alfred"}},
			JSONOutput: helpers.M{"rows": []helpers.M{
				{"tenant": "alfred", "bytes": 30},
			}},
		}, {
			Description: "another user sees theirs",
			URL:         "/api/v0/console/scoped",
			Header:      http.Header{"Remote-User": []string{"bernard"}},
			JSONOutput: helpers.M{"rows": []helpers.M{
				{"tenant": "bernard", "bytes": 100},
			}},
		}, {
			Description: "an unknown user sees nothing",
			URL:         "/api/v0/console/scoped",
			Header:      http.Header{"Remote-User": []string{"charles"}},
			JSONOutput:  helpers.M{"rows": []helpers.M{}},
		},
	})
}

// TestRefreshFlowsTablesUnderRowPolicy checks the refresh keeps working when a
// row policy relying on the setting guards the flows tables: it reads
// system.parts, which the policy does not cover, and gets the real oldest
// timestamp rather than an error.
func TestRefreshFlowsTablesUnderRowPolicy(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r, false)
	h := httpserver.NewMock(t, r)
	c := newTestComponent(t, r, chComponent, h, authentication.NewMock(t, r), "SQL_akvorado_user")

	db := chComponent.DatabaseName()
	policy := fmt.Sprintf("scope_%s_flows", db)
	for _, query := range []string{
		fmt.Sprintf(`CREATE TABLE %s.flows (TimeReceived DateTime, SrcNetTenant String)
                     ENGINE = MergeTree
                     PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL 3600 second))
                     ORDER BY TimeReceived`, db),
		// Explicit time zone: a bare literal is read in the server's one.
		fmt.Sprintf(`INSERT INTO %s.flows VALUES
                     (toDateTime('2022-04-10 15:45:10', 'UTC'), 'alfred'),
                     (toDateTime('2022-04-20 10:00:00', 'UTC'), 'bernard')`, db),
		fmt.Sprintf(`CREATE ROW POLICY %s ON %s.flows FOR SELECT
                     USING SrcNetTenant = getSetting('SQL_akvorado_user') TO default`, policy, db),
	} {
		if err := chComponent.Exec(t.Context(), query); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", query, err)
		}
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		chComponent.Exec(ctx, fmt.Sprintf("DROP ROW POLICY %s ON %s.flows", policy, db))
	})

	// Querying the table directly is what a fail-closed policy forbids here.
	var direct []struct {
		T time.Time `ch:"t"`
	}
	if err := chComponent.Select(t.Context(), &direct,
		fmt.Sprintf(`SELECT MIN(TimeReceived) AS t FROM %s.flows`, db)); err == nil {
		t.Fatalf("direct query on the table succeeded, the policy is not fail-closed")
	}

	if err := c.refreshFlowsTables(); err != nil {
		t.Fatalf("refreshFlowsTables() error:\n%+v", err)
	}
	// DateTime values come back in the time zone of the server: compare the
	// instant, not its representation.
	got := c.flowsTables
	if len(got) != 1 || got[0].Name != "flows" || got[0].Resolution != 0 ||
		!got[0].Oldest.Equal(time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)) {
		t.Fatalf("refreshFlowsTables() got %+v, expected flows starting 2022-04-10 15:45:10 UTC", got)
	}
}
