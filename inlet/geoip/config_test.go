// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

func TestConfigurationUnmarshallerHook(t *testing.T) {
	cases := []struct {
		Description string
		Input       gin.H
		Output      Configuration
		Error       bool
	}{
		{
			Description: "nil",
			Input:       nil,
		}, {
			Description: "empty",
			Input:       gin.H{},
		}, {
			Description: "no country-database, no geoip-database",
			Input: gin.H{
				"asn-database": "something",
				"optional":     true,
			},
			Output: Configuration{
				ASNDatabase: "something",
				Optional:    true,
			},
		}, {
			Description: "country-database, no geoip-database",
			Input: gin.H{
				"asn-database":     "something",
				"country-database": "something else",
			},
			Output: Configuration{
				ASNDatabase: "something",
				GeoDatabase: "something else",
			},
		}, {
			Description: "no country-database, geoip-database",
			Input: gin.H{
				"asn-database": "something",
				"geo-database": "something else",
			},
			Output: Configuration{
				ASNDatabase: "something",
				GeoDatabase: "something else",
			},
		}, {
			Description: "both country-database, geoip-database",
			Input: gin.H{
				"asn-database":     "something",
				"geo-database":     "something else",
				"country-database": "another value",
			},
			Error: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var got Configuration
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				Result:      &got,
				ErrorUnused: true,
				Metadata:    nil,
				MatchName:   helpers.MapStructureMatchName,
				DecodeHook:  ConfigurationUnmarshallerHook(),
			})
			if err != nil {
				t.Fatalf("NewDecoder() error:\n%+v", err)
			}
			err = decoder.Decode(tc.Input)
			if err != nil && !tc.Error {
				t.Fatalf("Decode() error:\n%+v", err)
			} else if err == nil && tc.Error {
				t.Fatal("Decode() did not return an error")
			} else if diff := helpers.Diff(got, tc.Output); diff != "" {
				t.Fatalf("Decode() (-got, +want):\n%s", diff)
			}
		})
	}
}
