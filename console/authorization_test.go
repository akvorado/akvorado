// SPDX-FileCopyrightText: 2026 Gabriel ROUSSEAU (bolemo)
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/google/go-cmp/cmp"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
)

func init() {
	// Two rules are equal when they have the same source.
	helpers.RegisterCmpOption(cmp.Comparer(func(a, b AuthorizationRule) bool {
		return a.String() == b.String()
	}))
}

// parseRules compiles the provided rules, failing the test on error.
func parseRules(t *testing.T, rules ...string) []AuthorizationRule {
	t.Helper()
	parsed := make([]AuthorizationRule, len(rules))
	for idx, rule := range rules {
		if err := parsed[idx].UnmarshalText([]byte(rule)); err != nil {
			t.Fatalf("UnmarshalText(%q) error:\n%+v", rule, err)
		}
	}
	return parsed
}

// newTestComponent builds a console component around the provided ClickHouse,
// with the provided authorization rules.
func newTestComponent(t *testing.T, r *reporter.Reporter, ch *clickhousedb.Component, h *httpserver.Component, rules ...string) *Component {
	t.Helper()
	config := DefaultConfiguration()
	config.Authorization.Rules = parseRules(t, rules...)
	c, err := New(r, config, Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: ch,
		Clock:        clock.NewMock(),
		Auth:         authentication.NewMock(t, r),
		Database:     database.NewMock(t, r, database.DefaultConfiguration()),
		Schema:       schema.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}

func TestAuthorizationRuleUnmarshalText(t *testing.T) {
	cases := []struct {
		Description string
		Rule        string
		Filters     []string
		Error       string
	}{
		{
			Description: "allow",
			Rule:        `Login == "alfred" && Allow()`,
		}, {
			Description: "deny",
			Rule:        `Deny()`,
		}, {
			Description: "filter without placeholder",
			Rule:        `SetFilter("SrcNetTenant = 'acme'")`,
			Filters:     []string{"SrcNetTenant = 'acme'"},
		}, {
			Description: "filter with placeholders",
			Rule:        `"customers" in Groups && SetFilter("SrcNetTenant = ? OR DstNetTenant = ?", Login, Login)`,
			Filters:     []string{"SrcNetTenant = ? OR DstNetTenant = ?"},
		}, {
			Description: "filter with a list",
			Rule:        `SetFilter("SrcNetTenant IN (?)", Groups)`,
			Filters:     []string{"SrcNetTenant IN (?)"},
		}, {
			Description: "several filters",
			Rule:        `Email endsWith "@acme.com" ? SetFilter("SrcNetTenant = ?", "acme") : SetFilter("SrcNetTenant = ?", Login)`,
			Filters:     []string{"SrcNetTenant = ?", "SrcNetTenant = ?"},
		}, {
			Description: "question mark inside a literal is not a placeholder",
			Rule:        `SetFilter("SrcNetName = '?' AND DstNetTenant = ?", Login)`,
			Filters:     []string{"SrcNetName = '?' AND DstNetTenant = ?"},
		}, {
			Description: "syntax error",
			Rule:        `Login == "alfred" &&`,
			Error:       "cannot compile",
		}, {
			Description: "unknown function",
			Rule:        `Reject()`,
			Error:       "unknown name Reject",
		}, {
			Description: "not a boolean",
			Rule:        `Login`,
			Error:       "expected bool",
		}, {
			Description: "missing argument",
			Rule:        `SetFilter("SrcNetTenant = ? OR DstNetTenant = ?", Login)`,
			Error:       "2 placeholders but 1 arguments",
		}, {
			Description: "extra argument",
			Rule:        `SetFilter("SrcNetTenant = ?", Login, Login)`,
			Error:       "1 placeholders but 2 arguments",
		}, {
			Description: "filter is not a literal",
			Rule:        `SetFilter(Login)`,
			Error:       "must be a string literal",
		}, {
			Description: "no filter",
			Rule:        `SetFilter()`,
			Error:       "not enough arguments",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var rule AuthorizationRule
			err := rule.UnmarshalText([]byte(tc.Rule))
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("UnmarshalText() did not return an error")
				}
				if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("UnmarshalText() error %q does not contain %q", err.Error(), tc.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("UnmarshalText() error:\n%+v", err)
			}
			if diff := helpers.Diff(rule.filters, tc.Filters); diff != "" {
				t.Fatalf("UnmarshalText() filters (-got, +want):\n%s", diff)
			}
			if diff := helpers.Diff(rule.String(), tc.Rule); diff != "" {
				t.Fatalf("String() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestAuthorizationConfigurationDecode(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:   "no rule",
			Initial:       func() any { return AuthorizationConfiguration{} },
			Configuration: func() any { return helpers.M{} },
			Expected:      AuthorizationConfiguration{},
		}, {
			Description: "rules",
			Initial:     func() any { return AuthorizationConfiguration{} },
			Configuration: func() any {
				return helpers.M{
					"rules": []string{
						`Login == "admin" && Allow()`,
						`"customers" in Groups && SetFilter("SrcNetTenant = ?", Login)`,
					},
				}
			},
			Expected: AuthorizationConfiguration{
				Rules: parseRules(t,
					`Login == "admin" && Allow()`,
					`"customers" in Groups && SetFilter("SrcNetTenant = ?", Login)`),
			},
		}, {
			Description: "invalid rule",
			Initial:     func() any { return AuthorizationConfiguration{} },
			Configuration: func() any {
				return helpers.M{"rules": []string{`SetFilter(Login)`}}
			},
			Error: true,
		},
	})
}

func TestBindFilter(t *testing.T) {
	cases := []struct {
		Description string
		Template    string
		Args        []any
		Expected    string
		Error       string
	}{
		{
			Description: "no placeholder",
			Template:    "SrcNetTenant = 'acme'",
			Expected:    "SrcNetTenant = 'acme'",
		}, {
			Description: "one placeholder",
			Template:    "SrcNetTenant = ?",
			Args:        []any{"acme"},
			Expected:    "SrcNetTenant = 'acme'",
		}, {
			Description: "two placeholders",
			Template:    "SrcNetTenant = ? OR DstNetTenant = ?",
			Args:        []any{"acme", "globex"},
			Expected:    "SrcNetTenant = 'acme' OR DstNetTenant = 'globex'",
		}, {
			Description: "quote and backslash in a value",
			Template:    "SrcNetTenant = ?",
			Args:        []any{`o'brien \ co`},
			Expected:    `SrcNetTenant = 'o\'brien \\ co'`,
		}, {
			Description: "value trying to escape the literal",
			Template:    "SrcNetTenant = ?",
			Args:        []any{"' OR 1 = 1 --"},
			Expected:    `SrcNetTenant = '\' OR 1 = 1 --'`,
		}, {
			Description: "list",
			Template:    "SrcNetTenant IN (?)",
			Args:        []any{[]string{"acme", "globex"}},
			Expected:    "SrcNetTenant IN ('acme', 'globex')",
		}, {
			Description: "empty list",
			Template:    "SrcNetTenant IN (?)",
			Args:        []any{[]string{}},
			Expected:    "SrcNetTenant IN ()",
		}, {
			Description: "placeholder inside single quotes",
			Template:    "SrcNetName = '?' AND DstNetTenant = ?",
			Args:        []any{"acme"},
			Expected:    "SrcNetName = '?' AND DstNetTenant = 'acme'",
		}, {
			Description: "placeholder inside double quotes",
			Template:    `SrcNetName = "?" AND DstNetTenant = ?`,
			Args:        []any{"acme"},
			Expected:    `SrcNetName = "?" AND DstNetTenant = 'acme'`,
		}, {
			Description: "escaped quote inside a literal",
			Template:    `SrcNetName = 'it\'s ?' AND DstNetTenant = ?`,
			Args:        []any{"acme"},
			Expected:    `SrcNetName = 'it\'s ?' AND DstNetTenant = 'acme'`,
		}, {
			Description: "missing argument",
			Template:    "SrcNetTenant = ? OR DstNetTenant = ?",
			Args:        []any{"acme"},
			Error:       "2 placeholders but 1 arguments",
		}, {
			Description: "extra argument",
			Template:    "SrcNetTenant = ?",
			Args:        []any{"acme", "globex"},
			Error:       "1 placeholders but 2 arguments",
		}, {
			Description: "unsupported type",
			Template:    "SrcNetTenant = ?",
			Args:        []any{42},
			Error:       "unsupported type int",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			got, err := bindFilter(tc.Template, tc.Args)
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("bindFilter() did not return an error")
				}
				if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("bindFilter() error %q does not contain %q", err.Error(), tc.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("bindFilter() error:\n%+v", err)
			}
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Fatalf("bindFilter() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestAuthorizationDecide(t *testing.T) {
	rules := []string{
		`Login == "admin" && Allow()`,
		`Login == "nobody" && Deny()`,
		`"customers" in Groups && SetFilter("SrcNetTenant = ? OR DstNetTenant = ?", Login, Login)`,
		`"resellers" in Groups && SetFilter("SrcNetTenant IN (?)", Groups)`,
		`Email endsWith "@acme.com" && SetFilter("SrcNetTenant = 'acme'")`,
	}
	cases := []struct {
		Description string
		User        authentication.UserInformation
		Expected    authorizationDecision
	}{
		{
			Description: "allowed",
			User:        authentication.UserInformation{Login: "admin"},
			Expected:    authorizationDecision{taken: true},
		}, {
			Description: "denied",
			User:        authentication.UserInformation{Login: "nobody"},
			Expected:    authorizationDecision{taken: true, denied: true},
		}, {
			Description: "no rule decides",
			User:        authentication.UserInformation{Login: "alfred"},
			Expected:    authorizationDecision{},
		}, {
			Description: "filter on login",
			User:        authentication.UserInformation{Login: "alfred", Groups: []string{"customers"}},
			Expected: authorizationDecision{
				taken:  true,
				filter: "SrcNetTenant = 'alfred' OR DstNetTenant = 'alfred'",
			},
		}, {
			Description: "filter on groups",
			User:        authentication.UserInformation{Login: "bernard", Groups: []string{"resellers", "acme", "globex"}},
			Expected: authorizationDecision{
				taken:  true,
				filter: "SrcNetTenant IN ('resellers', 'acme', 'globex')",
			},
		}, {
			Description: "first rule wins",
			User:        authentication.UserInformation{Login: "admin", Groups: []string{"customers"}},
			Expected:    authorizationDecision{taken: true},
		}, {
			Description: "first filter wins",
			User:        authentication.UserInformation{Login: "charles", Groups: []string{"resellers", "customers"}},
			Expected: authorizationDecision{
				taken:  true,
				filter: "SrcNetTenant = 'charles' OR DstNetTenant = 'charles'",
			},
		}, {
			Description: "filter without placeholder",
			User:        authentication.UserInformation{Login: "david", Email: "david@acme.com"},
			Expected:    authorizationDecision{taken: true, filter: "SrcNetTenant = 'acme'"},
		},
	}
	config := AuthorizationConfiguration{Rules: parseRules(t, rules...)}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			got, err := config.decide(tc.User)
			if err != nil {
				t.Fatalf("decide() error:\n%+v", err)
			}
			if diff := helpers.Diff(got, tc.Expected, cmp.AllowUnexported(authorizationDecision{})); diff != "" {
				t.Fatalf("decide() (-got, +want):\n%s", diff)
			}
		})
	}
}

// TestAuthorizationRuleValidate checks the filters are checked against the
// schema when the console starts, not on each request.
func TestAuthorizationRuleValidate(t *testing.T) {
	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
	cases := []struct {
		Description string
		Rule        string
		Error       string
	}{
		{
			Description: "no filter",
			Rule:        `Allow()`,
		}, {
			Description: "valid filter",
			Rule:        `SetFilter("SrcNetTenant = ? OR DstNetTenant = ?", Login, Login)`,
		}, {
			Description: "valid filter with a list",
			Rule:        `SetFilter("SrcNetTenant IN (?)", Groups)`,
		}, {
			Description: "valid filter with a dictionary",
			Rule:        `SetFilter("SrcNetName = ?", Login)`,
		}, {
			Description: "unknown column",
			Rule:        `SetFilter("Tenant = ?", Login)`,
			Error:       "cannot parse filter",
		}, {
			Description: "column not accepting a string",
			Rule:        `SetFilter("SrcAS = ?", Login)`,
			Error:       "cannot parse filter",
		}, {
			Description: "column only in the main table",
			Rule:        `SetFilter("SrcAddr = 192.0.2.1")`,
			Error:       "only available in the main flows table",
		}, {
			Description: "second filter invalid",
			Rule:        `Login == "a" ? SetFilter("SrcNetTenant = ?", Login) : SetFilter("Nope = ?", Login)`,
			Error:       "cannot parse filter",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			config := DefaultConfiguration()
			config.Authorization.Rules = parseRules(t, tc.Rule)
			_, err := New(r, config, Dependencies{
				Daemon:       daemon.NewMock(t),
				HTTP:         httpserver.NewMock(t, r),
				ClickHouseDB: ch,
				Clock:        clock.NewMock(),
				Auth:         authentication.NewMock(t, r),
				Database:     database.NewMock(t, r, database.DefaultConfiguration()),
				Schema:       schema.NewMock(t),
			})
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("New() with rule %q did not return an error", tc.Rule)
				}
				if !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("New() error %q does not contain %q", err.Error(), tc.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error:\n%+v", err)
			}
		})
	}
}

func TestAdditionalTableFilters(t *testing.T) {
	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
	c := newTestComponent(t, r, ch, httpserver.NewMock(t, r), `Allow()`)
	t.Run("tables from the context", func(t *testing.T) {
		ctx := contextWithFlowsTables(t.Context(), []flowsTable{
			{"flows", 0, time.Time{}},
			{"flows_1m0s", time.Minute, time.Time{}},
		})
		got := c.additionalTableFilters(ctx, `SrcNetTenant = 'o\'brien'`)
		expected := `{'default.flows': 'SrcNetTenant = \'o\\\'brien\'', ` +
			`'default.flows_1m0s': 'SrcNetTenant = \'o\\\'brien\''}`
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Fatalf("additionalTableFilters() (-got, +want):\n%s", diff)
		}
	})
	t.Run("no tables in the context", func(t *testing.T) {
		// A handler reached without withFlowsTables() must not fall back on the
		// current list.
		defer func() {
			if recover() == nil {
				t.Fatal("additionalTableFilters() did not panic")
			}
		}()
		c.additionalTableFilters(t.Context(), `SrcNetTenant = 'acme'`)
	})
}

// TestAuthorizationMiddleware checks the decisions reach the user, and that
// responses are shared between users with the same filter only.
func TestAuthorizationMiddleware(t *testing.T) {
	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
	h := httpserver.NewMock(t, r)
	c := newTestComponent(t, r, ch, h,
		`Login == "admin" && Allow()`,
		`Login == "nobody" && Deny()`,
		`"customers" in Groups && SetFilter("SrcNetTenant = ?", Login)`,
		`"acme" in Groups && SetFilter("SrcNetTenant = ?", "acme")`,
	)

	// A cached handler counting how many times it was really called. Two
	// users sharing a cached response get the same count.
	var calls atomic.Int64
	h.APIRouter.GET("/api/v0/console/counter",
		func(w http.ResponseWriter, _ *http.Request) {
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{"calls": calls.Add(1)})
		},
		c.d.Auth.UserAuthentication(), c.withFlowsTables(), c.authorization(), h.CacheByRequestPath(time.Minute))

	headers := func(login, groups string) http.Header {
		return http.Header{"Remote-User": []string{login}, "Remote-Groups": []string{groups}}
	}
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "allowed user",
			URL:         "/api/v0/console/counter",
			Header:      headers("admin", ""),
			JSONOutput:  helpers.M{"calls": 1},
		}, {
			Description: "denied user",
			URL:         "/api/v0/console/counter",
			Header:      headers("nobody", ""),
			StatusCode:  http.StatusForbidden,
			JSONOutput:  helpers.M{"message": "Access denied."},
		}, {
			Description: "user matching no rule",
			URL:         "/api/v0/console/counter",
			Header:      headers("alfred", ""),
			StatusCode:  http.StatusForbidden,
			JSONOutput:  helpers.M{"message": "Access denied."},
		}, {
			Description: "default user matching no rule",
			URL:         "/api/v0/console/counter",
			StatusCode:  http.StatusForbidden,
			JSONOutput:  helpers.M{"message": "Access denied."},
		}, {
			Description: "restricted user does not share with the allowed one",
			URL:         "/api/v0/console/counter",
			Header:      headers("alfred", "customers"),
			JSONOutput:  helpers.M{"calls": 2},
		}, {
			Description: "restricted user shares with themselves",
			URL:         "/api/v0/console/counter",
			Header:      headers("alfred", "customers"),
			JSONOutput:  helpers.M{"calls": 2},
		}, {
			Description: "another filter is another entry",
			URL:         "/api/v0/console/counter",
			Header:      headers("bernard", "customers"),
			JSONOutput:  helpers.M{"calls": 3},
		}, {
			Description: "users with the same filter share",
			URL:         "/api/v0/console/counter",
			Header:      headers("charles", "acme"),
			JSONOutput:  helpers.M{"calls": 4},
		}, {
			Description: "users with the same filter share (second user)",
			URL:         "/api/v0/console/counter",
			Header:      headers("david", "acme"),
			JSONOutput:  helpers.M{"calls": 4},
		}, {
			Description: "allowed user still gets the unrestricted entry",
			URL:         "/api/v0/console/counter",
			Header:      headers("admin", ""),
			JSONOutput:  helpers.M{"calls": 1},
		},
	})
}

// TestAuthorizationFilterApplied checks the filter really restricts what a
// user gets from ClickHouse, on every flows table. This is the test proving the
// feature: ClickHouse silently ignores a filter on a key matching no table, so
// only the result tells.
func TestAuthorizationFilterApplied(t *testing.T) {
	r := reporter.NewMock(t)
	chComponent := clickhousedb.SetupClickHouse(t, r, false)
	h := httpserver.NewMock(t, r)
	c := newTestComponent(t, r, chComponent, h,
		`Login == "admin" && Allow()`,
		`"customers" in Groups && SetFilter("SrcNetTenant = ? OR DstNetTenant = ?", Login, Login)`,
		`"resellers" in Groups && SetFilter("SrcNetTenant IN (?)", Groups)`,
	)

	// Two flows tables, as the console would find them.
	db := chComponent.DatabaseName()
	for _, query := range []string{
		fmt.Sprintf(`CREATE TABLE %s.flows (TimeReceived DateTime, SrcNetTenant String,
                     DstNetTenant String, Bytes UInt64)
                     ENGINE = MergeTree ORDER BY TimeReceived`, db),
		fmt.Sprintf(`CREATE TABLE %s.flows_1m0s AS %s.flows
                     ENGINE = MergeTree ORDER BY TimeReceived`, db, db),
		fmt.Sprintf(`INSERT INTO %s.flows VALUES
                     (now(), 'alfred', 'bernard', 10),
                     (now(), 'bernard', 'alfred', 20),
                     (now(), 'charles', 'charles', 100),
                     (now(), 'o\'brien', 'o\'brien', 7)`, db),
		fmt.Sprintf(`INSERT INTO %s.flows_1m0s VALUES
                     (now(), 'alfred', 'bernard', 1),
                     (now(), 'bernard', 'alfred', 2),
                     (now(), 'charles', 'charles', 50)`, db),
	} {
		if err := chComponent.Exec(t.Context(), query); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", query, err)
		}
	}
	if err := c.refreshFlowsTables(); err != nil {
		t.Fatalf("refreshFlowsTables() error:\n%+v", err)
	}
	if len(c.flowsTables) != 2 {
		t.Fatalf("refreshFlowsTables() found %d tables, expected 2", len(c.flowsTables))
	}

	// A handler querying ClickHouse the way the real ones do, from the context
	// of the request, with a bare table name and a qualified one.
	h.APIRouter.GET("/api/v0/console/{table}",
		func(w http.ResponseWriter, req *http.Request) {
			var results []struct {
				Bytes uint64 `ch:"Bytes"`
			}
			err := chComponent.Select(req.Context(), &results,
				fmt.Sprintf(`SELECT SUM(Bytes) AS Bytes FROM %s`, req.PathValue("table")))
			if err != nil {
				httpserver.WriteJSON(w, http.StatusInternalServerError,
					helpers.M{"message": err.Error()})
				return
			}
			httpserver.WriteJSON(w, http.StatusOK, helpers.M{"bytes": results[0].Bytes})
		},
		c.d.Auth.UserAuthentication(), c.withFlowsTables(), c.authorization())

	headers := func(login, groups string) http.Header {
		return http.Header{"Remote-User": []string{login}, "Remote-Groups": []string{groups}}
	}
	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "allowed user sees everything",
			URL:         "/api/v0/console/flows",
			Header:      headers("admin", ""),
			JSONOutput:  helpers.M{"bytes": 137},
		}, {
			Description: "user matching no rule",
			URL:         "/api/v0/console/flows",
			Header:      headers("alfred", ""),
			StatusCode:  http.StatusForbidden,
			JSONOutput:  helpers.M{"message": "Access denied."},
		}, {
			Description: "customer sees their flows",
			URL:         "/api/v0/console/flows",
			Header:      headers("alfred", "customers"),
			JSONOutput:  helpers.M{"bytes": 30},
		}, {
			Description: "another customer sees theirs",
			URL:         "/api/v0/console/flows",
			Header:      headers("charles", "customers"),
			JSONOutput:  helpers.M{"bytes": 100},
		}, {
			Description: "unknown customer sees nothing",
			URL:         "/api/v0/console/flows",
			Header:      headers("edward", "customers"),
			JSONOutput:  helpers.M{"bytes": 0},
		}, {
			Description: "quote in the login stays in the value",
			URL:         "/api/v0/console/flows",
			Header:      headers("o'brien", "customers"),
			JSONOutput:  helpers.M{"bytes": 7},
		}, {
			Description: "reseller sees the tenants of their groups",
			URL:         "/api/v0/console/flows",
			Header:      headers("francis", "resellers, alfred, bernard"),
			JSONOutput:  helpers.M{"bytes": 30},
		}, {
			Description: "consolidated table is filtered too",
			URL:         "/api/v0/console/flows_1m0s",
			Header:      headers("alfred", "customers"),
			JSONOutput:  helpers.M{"bytes": 3},
		}, {
			Description: "qualified table is filtered too",
			URL:         fmt.Sprintf("/api/v0/console/%s.flows_1m0s", db),
			Header:      headers("charles", "customers"),
			JSONOutput:  helpers.M{"bytes": 50},
		}, {
			Description: "allowed user sees the whole consolidated table",
			URL:         "/api/v0/console/flows_1m0s",
			Header:      headers("admin", ""),
			JSONOutput:  helpers.M{"bytes": 53},
		},
	})
}
