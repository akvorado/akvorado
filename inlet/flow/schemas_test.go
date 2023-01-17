// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestHTTPEndpoints(t *testing.T) {
	r := reporter.NewMock(t)
	c := NewMock(t, r, DefaultConfiguration())

	cases := helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/inlet/flow/schema.proto",
			ContentType: "text/plain",
			FirstLines: []string{
				"",
				`syntax = "proto3";`,
			},
		},
	}

	helpers.TestHTTPEndpoints(t, c.d.HTTP.LocalAddr(), cases)
}
