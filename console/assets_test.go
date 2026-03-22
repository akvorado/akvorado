// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"testing"

	"akvorado/common/helpers"
)

func TestServeAssets(t *testing.T) {
	for _, live := range []bool{false, true} {
		var name string
		switch live {
		case true:
			name = "livefs"
		case false:
			name = "embeddedfs"
		}
		t.Run(name, func(t *testing.T) {
			conf := DefaultConfiguration()
			conf.ServeLiveFS = live
			_, h, _, _ := NewMock(t, conf)

			helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
				{
					URL:         "/",
					ContentType: "text/html; charset=utf-8",
					// Verify the <base href="/"> tag is injected at the top of <head>.
					FirstLines: []string{
						"<!doctype html>",
						`<html lang="en" class="h-full">`,
						"  <head>",
						`    <base href="/" />`,
					},
				}, {
					URL:         "/something",
					ContentType: "text/html; charset=utf-8",
					FirstLines:  []string{"<!doctype html>"},
				}, {
					URL:         "/assets/akvorado-DXhK_DdK.svg",
					ContentType: "image/svg+xml",
					FirstLines:  []string{`<?xml version="1.0" encoding="UTF-8" standalone="no"?>`},
				}, {
					URL:         "/assets/somethingelse.svg",
					StatusCode:  404,
					ContentType: "text/plain; charset=utf-8",
					FirstLines:  []string{"404 page not found"},
				},
			})
		})
	}
}

func TestServeAssetsWithURLPrefix(t *testing.T) {
	conf := DefaultConfiguration()
	conf.URLPrefix = "/akvorado/"
	_, h, _, _ := NewMock(t, conf)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "index.html injects correct base href",
			URL:         "/",
			ContentType: "text/html; charset=utf-8",
			// The <base href> must reflect the configured prefix so that
			// relative asset URLs (./assets/...) and API calls (api/v0/...)
			// resolve to /akvorado/assets/... and /akvorado/api/v0/... in the
			// browser when a stripping reverse proxy is in use.
			FirstLines: []string{
				"<!doctype html>",
				`<html lang="en" class="h-full">`,
				"  <head>",
				`    <base href="/akvorado/" />`,
			},
		},
	})
}

func TestURLPrefixNormalisation(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "/"},
		{"/", "/"},
		{"/akvorado/", "/akvorado/"},
		// Missing leading slash
		{"akvorado/", "/akvorado/"},
		// Missing trailing slash
		{"/akvorado", "/akvorado/"},
		// Missing both slashes
		{"akvorado", "/akvorado/"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			conf := DefaultConfiguration()
			conf.URLPrefix = tc.input
			_, h, _, _ := NewMock(t, conf)

			helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
				{
					URL:         "/",
					ContentType: "text/html; charset=utf-8",
					FirstLines: []string{
						"<!doctype html>",
						`<html lang="en" class="h-full">`,
						"  <head>",
						fmt.Sprintf(`    <base href=%q />`, tc.want),
					},
				},
			})
		})
	}
}
