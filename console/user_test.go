package console

import (
	netHTTP "net/http"
	"testing"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestUserHandler(t *testing.T) {
	r := reporter.NewMock(t)
	ch, _ := clickhousedb.NewMock(t, r)
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

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			Description: "user info, no user logged in",
			URL:         "/api/v0/console/user/info",
			StatusCode:  401,
			JSONOutput:  gin.H{"message": "No user logged in."},
		}, {
			Description: "user info, minimal user logged in",
			URL:         "/api/v0/console/user/info",
			Header: func() netHTTP.Header {
				headers := make(netHTTP.Header)
				headers.Add("X-Akvorado-User-Login", "alfred")
				return headers
			}(),
			StatusCode: 200,
			JSONOutput: gin.H{
				"login":      "alfred",
				"avatar-url": "/api/v0/console/user/avatar",
			},
		}, {
			Description: "user info, complete user logged in",
			URL:         "/api/v0/console/user/info",
			Header: func() netHTTP.Header {
				headers := make(netHTTP.Header)
				headers.Add("X-Akvorado-User-Login", "alfred")
				headers.Add("X-Akvorado-User-Name", "Alfred Pennyworth")
				headers.Add("X-Akvorado-User-Email", "alfred@batman.com")
				headers.Add("X-Akvorado-User-Avatar", "https://some.example.com/avatar.png")
				headers.Add("X-Akvorado-User-Logout", "/logout")
				return headers
			}(),
			StatusCode: 200,
			JSONOutput: gin.H{
				"login":      "alfred",
				"name":       "Alfred Pennyworth",
				"email":      "alfred@batman.com",
				"avatar-url": "https://some.example.com/avatar.png",
				"logout-url": "/logout",
			},
		}, {
			Description: "user info, invalid user logged in",
			URL:         "/api/v0/console/user/info",
			Header: func() netHTTP.Header {
				headers := make(netHTTP.Header)
				headers.Add("X-Akvorado-User-Login", "alfred")
				headers.Add("X-Akvorado-User-Email", "alfrednooo")
				return headers
			}(),
			StatusCode: 401,
			JSONOutput: gin.H{"message": "No user logged in."},
		}, {
			Description: "avatar, no user logged in",
			URL:         "/api/v0/console/user/avatar",
			StatusCode:  401,
			JSONOutput:  gin.H{"message": "No user logged in."},
		}, {
			Description: "avatar, simple user",
			URL:         "/api/v0/console/user/avatar",
			Header: func() netHTTP.Header {
				headers := make(netHTTP.Header)
				headers.Add("X-Akvorado-User-Login", "alfred")
				return headers
			}(),
			ContentType: "image/png",
			StatusCode:  200,
		},
	})
}
