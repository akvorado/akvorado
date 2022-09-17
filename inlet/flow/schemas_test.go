// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"fmt"
	"strconv"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"

	"github.com/gin-gonic/gin"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration())
	versions := gin.H{}
	for i := 0; i < CurrentSchemaVersion+1; i++ {
		versions[strconv.Itoa(i)] = fmt.Sprintf("/api/v0/inlet/flow/schema-%d.proto", i)
	}

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
				"current-version": CurrentSchemaVersion,
				"versions":        versions,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), cases)
}
