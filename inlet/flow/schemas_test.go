// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration())

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/inlet/flow/schema-0.proto",
			ContentType: "text/plain",
			FirstLines: []string{
				`syntax = "proto3";`,
				`package decoder;`,
			},
		}, {
			URL: "/api/v0/inlet/flow/schemas.json",
			JSONOutput: gin.H{
				"current-version": 1,
				"versions": gin.H{
					"0": "/api/v0/inlet/flow/schema-0.proto",
					"1": "/api/v0/inlet/flow/schema-1.proto",
				},
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.Address, cases)
}
