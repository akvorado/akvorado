// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
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
					FirstLines:  []string{"<!doctype html>"},
				}, {
					URL:         "/something",
					ContentType: "text/html; charset=utf-8",
					FirstLines:  []string{"<!doctype html>"},
				}, {
					URL:         "/assets/akvorado-14Svw3Su.svg",
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
