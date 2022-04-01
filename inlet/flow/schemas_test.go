package flow

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration)

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/inlet/flow/schema-0.proto",
			ContentType: "text/plain",
			FirstLines: []string{
				`syntax = "proto3";`,
				`package decoder;`,
			},
		}, {
			URL:         "/api/v0/inlet/flow/schemas.json",
			ContentType: "application/json",
			FirstLines: []string{
				`{`,
				` "current_version": 1,`,
				` "versions": {`,
				`  "0": "/api/v0/inlet/flow/schema-0.proto",`,
				`  "1": "/api/v0/inlet/flow/schema-1.proto"`,
				` }`,
				`}`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.Address, cases)
}
