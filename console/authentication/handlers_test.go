// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package authentication

import (
	netHTTP "net/http"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestUserHandler(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration())
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Configure the two endpoints
	endpoint := h.GinRouter.Group("/api/v0/console/user", c.UserAuthentication())
	endpoint.GET("/info", c.UserInfoHandlerFunc)
	endpoint.GET("/avatar", c.UserAvatarHandlerFunc)

	t.Run("default user configured", func(t *testing.T) {
		helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "user info, no user logged in",
				URL:         "/api/v0/console/user/info",
				StatusCode:  200,
				JSONOutput:  gin.H{"login": "__default", "name": "Default User"},
			}, {
				Description: "user info, minimal user logged in",
				URL:         "/api/v0/console/user/info",
				Header: func() netHTTP.Header {
					headers := make(netHTTP.Header)
					headers.Add("Remote-User", "alfred")
					return headers
				}(),
				StatusCode: 200,
				JSONOutput: gin.H{
					"login": "alfred",
				},
			}, {
				Description: "user info, complete user logged in",
				URL:         "/api/v0/console/user/info",
				Header: func() netHTTP.Header {
					headers := make(netHTTP.Header)
					headers.Add("Remote-User", "alfred")
					headers.Add("Remote-Name", "Alfred Pennyworth")
					headers.Add("Remote-Email", "alfred@batman.com")
					headers.Add("X-Logout-URL", "/logout")
					return headers
				}(),
				StatusCode: 200,
				JSONOutput: gin.H{
					"login":      "alfred",
					"name":       "Alfred Pennyworth",
					"email":      "alfred@batman.com",
					"logout-url": "/logout",
				},
			}, {
				Description: "user info, invalid user logged in",
				URL:         "/api/v0/console/user/info",
				Header: func() netHTTP.Header {
					headers := make(netHTTP.Header)
					headers.Add("Remote-User", "alfred")
					headers.Add("Remote-Email", "alfrednooo")
					return headers
				}(),
				StatusCode: 200,
				JSONOutput: gin.H{"login": "__default", "name": "Default User"},
			}, {
				Description: "avatar, no user logged in",
				URL:         "/api/v0/console/user/avatar",
				ContentType: "image/png",
				StatusCode:  200,
			}, {
				Description: "avatar, simple user",
				URL:         "/api/v0/console/user/avatar",
				Header: func() netHTTP.Header {
					headers := make(netHTTP.Header)
					headers.Add("Remote-User", "alfred")
					return headers
				}(),
				ContentType: "image/png",
				StatusCode:  200,
			}, {
				Description: "avatar, simple user, etag",
				URL:         "/api/v0/console/user/avatar",
				Header: func() netHTTP.Header {
					headers := make(netHTTP.Header)
					headers.Add("Remote-User", "alfred")
					headers.Add("If-None-Match", `"b2e72a535032fa89"`)
					return headers
				}(),
				StatusCode: 304,
			},
		})
	})

	t.Run("no default user", func(t *testing.T) {
		c.config.DefaultUser.Login = ""
		helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
			{
				Description: "user info, no user logged in",
				URL:         "/api/v0/console/user/info",
				StatusCode:  401,
				JSONOutput:  gin.H{"message": "No user logged in."},
			}, {
				Description: "user info, invalid user logged in",
				URL:         "/api/v0/console/user/info",
				Header: func() netHTTP.Header {
					headers := make(netHTTP.Header)
					headers.Add("Remote-User", "alfred")
					headers.Add("Remote-Email", "alfrednooo")
					return headers
				}(),
				StatusCode: 401,
				JSONOutput: gin.H{"message": "No user logged in."},
			}, {
				Description: "avatar, no user logged in",
				URL:         "/api/v0/console/user/avatar",
				StatusCode:  401,
				JSONOutput:  gin.H{"message": "No user logged in."},
			},
		})
	})
}
