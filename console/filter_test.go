package console

import (
	"testing"

	"github.com/gin-gonic/gin"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
)

func TestFilterHandlers(t *testing.T) {
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
			URL:       "/api/v0/console/filter/validate",
			JSONInput: gin.H{"filter": `InIfName = "Gi0/0/0/1"`},
			JSONOutput: gin.H{
				"message": "ok",
				"parsed":  `InIfName = 'Gi0/0/0/1'`},
		},
		{
			URL:        "/api/v0/console/filter/validate",
			StatusCode: 400,
			JSONInput:  gin.H{"filter": `InIfName = "`},
			JSONOutput: gin.H{
				"message": "at line 1, position 12: string literal not terminated",
				"errors": []gin.H{{
					"line":    1,
					"column":  12,
					"offset":  11,
					"message": "string literal not terminated",
				}},
			},
		},
	})
}
