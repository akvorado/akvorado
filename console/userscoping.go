// SPDX-FileCopyrightText: 2026 Gabriel ROUSSEAU (bolemo)
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"net/http"
	"regexp"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/httpserver"
	"akvorado/console/authentication"
)

// clickHouseSettingName matches the names ClickHouse accepts for a custom
// setting. The prefix still has to be one of the `custom_settings_prefixes' of
// the server, but this is not something we can check from here.
var clickHouseSettingName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// userSettings returns the ClickHouse settings carrying the identity of the
// user a request is served for. It returns nil when the feature is disabled.
func (c *Component) userSettings(login string) clickhouse.Settings {
	if c.config.ClickHouseUserSetting == "" {
		return nil
	}
	// CustomSetting is mandatory: without it, the value is sent as a builtin
	// setting and ClickHouse rejects it as an unknown one.
	return clickhouse.Settings{
		c.config.ClickHouseUserSetting: clickhouse.CustomSetting{Value: login},
	}
}

// userScoping is a middleware attaching the login of the authenticated user to
// every ClickHouse query made while serving the request. An operator can then
// use it in a ClickHouse row policy to restrict the flows this user is able to
// see. As responses become user-specific, it also makes the HTTP caches
// discriminate on the login.
//
// It does nothing when `clickhouse-user-setting' is not configured.
func (c *Component) userScoping() httpserver.Middleware {
	return func(next http.Handler) http.Handler {
		if c.config.ClickHouseUserSetting == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			login := authentication.UserFromContext(req.Context()).Login
			ctx := clickhouse.Context(req.Context(),
				clickhouse.WithSettings(c.userSettings(login)))
			ctx = httpserver.WithCacheKeyDiscriminator(ctx, login)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}
