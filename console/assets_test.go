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

			helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
				{
					URL:         "/",
					ContentType: "text/html; charset=utf-8",
					FirstLines:  []string{"<!DOCTYPE html>"},
				}, {
					URL:         "/something",
					ContentType: "text/html; charset=utf-8",
					FirstLines:  []string{"<!DOCTYPE html>"},
				}, {
					URL:         "/assets/akvorado.399701ee.svg",
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
