// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package orchestrator

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
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
	c.RegisterConfiguration(InletService, map[string]string{
		"hello": "Hello pal!",
		"bye":   "Goodbye pal!",
	})

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			URL:         "/api/v0/orchestrator/configuration/inlet",
			ContentType: "application/x-yaml; charset=utf-8",
			FirstLines: []string{
				`bye: Goodbye world!`,
				`hello: Hello world!`,
			},
		}, {
			URL:         "/api/v0/orchestrator/configuration/inlet/0",
			ContentType: "application/x-yaml; charset=utf-8",
			FirstLines: []string{
				`bye: Goodbye world!`,
				`hello: Hello world!`,
			},
		}, {
			URL:         "/api/v0/orchestrator/configuration/inlet/1",
			ContentType: "application/x-yaml; charset=utf-8",
			FirstLines: []string{
				`bye: Goodbye pal!`,
				`hello: Hello pal!`,
			},
		}, {
			URL:         "/api/v0/orchestrator/configuration/inlet/2",
			ContentType: "application/x-yaml; charset=utf-8",
			FirstLines: []string{
				`bye: Goodbye world!`,
				`hello: Hello world!`,
			},
		}, {
			URL:         "/api/v0/orchestrator/configuration/console/0",
			ContentType: "application/json; charset=utf-8",
			StatusCode:  404,
		},
	})
}
