package orchestrator

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestConfigurationEndpoint(t *testing.T) {
	r := reporter.NewMock(t)
	h := http.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		HTTP: h,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	c.RegisterConfiguration(InletService, map[string]string{
		"hello": "Hello world!",
		"bye":   "Goodbye world!",
	})

	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/orchestrator/broker/configuration/inlet",
			JSONOutput: gin.H{
				"bye":   "Goodbye world!",
				"hello": "Hello world!",
			},
		},
	})
}
