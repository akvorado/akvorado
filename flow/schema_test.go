package flow

import (
	"testing"

	"akvorado/helpers"
	"akvorado/reporter"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration)

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/flow/schema-0.proto",
			ContentType: "text/plain",
			FirstLines: []string{
				`syntax = "proto3";`,
				`package flow;`,
			},
		}, {
			URL:         "/api/v0/flow/schemas.json",
			ContentType: "application/json",
			FirstLines: []string{
				`{`,
				` "current_version": 0,`,
				` "versions": {`,
				`  "0": "/api/v0/flow/schema-0.proto"`,
				` }`,
				`}`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.Address, cases)
}
