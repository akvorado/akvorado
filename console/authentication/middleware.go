// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package authentication

import (
	"context"
	"net/http"
	"strings"

	"github.com/valyala/fasttemplate"

	"akvorado/common/helpers"
	"akvorado/common/httpserver"
)

// UserInformation contains information about the current user.
type UserInformation struct {
	Login     string `json:"login" validate:"required"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty" validate:"omitempty,email"`
	LogoutURL string `json:"logout-url,omitempty" validate:"omitempty,uri"`
	AvatarURL string `json:"avatar-url,omitempty" validate:"omitempty,uri"`
}

// userContextKey is the key under which the current user is stored in the
// request context.
type userContextKey struct{}

// UserFromContext retrieves the UserInformation stored by UserAuthentication.
// It panics if no user is present.
func UserFromContext(ctx context.Context) UserInformation {
	return ctx.Value(userContextKey{}).(UserInformation)
}

// UserAuthentication is a middleware to fill information about the current
// user. It does not really perform authentication but relies on HTTP headers.
func (c *Component) UserAuthentication() httpserver.Middleware {
	var logoutURLTmpl, avatarURLTmpl *fasttemplate.Template
	if c.config.LogoutURL != "" {
		logoutURLTmpl, _ = fasttemplate.NewTemplate(c.config.LogoutURL, "{{", "}}")
	}
	if c.config.AvatarURL != "" {
		avatarURLTmpl, _ = fasttemplate.NewTemplate(c.config.AvatarURL, "{{", "}}")
	}

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			info := c.userFromHeaders(req)
			if err := helpers.Validate.Struct(info); err != nil {
				if c.config.DefaultUser.Login == "" {
					httpserver.WriteJSON(w, http.StatusUnauthorized,
						helpers.M{"message": "No user logged in."})
					return
				}
				info = c.config.DefaultUser
			}
			data := map[string]any{
				"Login":     info.Login,
				"Name":      info.Name,
				"Email":     info.Email,
				"LogoutURL": info.LogoutURL,
				"AvatarURL": info.AvatarURL,
			}

			// Apply configured templates (they can access header values and choose to keep or override)
			if logoutURLTmpl != nil {
				var buf strings.Builder
				if _, err := logoutURLTmpl.Execute(&buf, data); err == nil {
					info.LogoutURL = buf.String()
				}
			}
			if avatarURLTmpl != nil {
				var buf strings.Builder
				if _, err := avatarURLTmpl.Execute(&buf, data); err == nil {
					info.AvatarURL = buf.String()
				}
			}

			ctx := context.WithValue(req.Context(), userContextKey{}, info)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// userFromHeaders builds a UserInformation from the request headers configured
// on the component. Empty header names are skipped.
func (c *Component) userFromHeaders(req *http.Request) UserInformation {
	get := func(name string) string {
		if name == "" {
			return ""
		}
		return req.Header.Get(name)
	}
	return UserInformation{
		Login:     get(c.config.Headers.Login),
		Name:      get(c.config.Headers.Name),
		Email:     get(c.config.Headers.Email),
		LogoutURL: get(c.config.Headers.LogoutURL),
		AvatarURL: get(c.config.Headers.AvatarURL),
	}
}
