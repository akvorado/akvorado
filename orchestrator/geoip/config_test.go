// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package geoip

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/gin-gonic/gin"
)

func TestConfigurationUnmarshallerHook(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description:   "nil",
			Initial:       func() any { return Configuration{} },
			Configuration: func() any { return nil },
			Expected:      Configuration{},
		}, {
			Description:   "empty",
			Initial:       func() any { return Configuration{} },
			Configuration: func() any { return gin.H{} },
			Expected:      Configuration{},
		}, {
			Description: "no country-database, no geoip-database",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"asn-database": []string{"something"},
					"optional":     true,
				}
			},
			Expected: Configuration{
				ASNDatabase: []string{"something"},
				Optional:    true,
			},
		}, {
			Description: "country-database, no geoip-database",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"asn-database":     []string{"something"},
					"country-database": []string{"something else"},
				}
			},
			Expected: Configuration{
				ASNDatabase: []string{"something"},
				GeoDatabase: []string{"something else"},
			},
		}, {
			Description: "no country-database, geoip-database",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"asn-database": []string{"something"},
					"geo-database": []string{"something else"},
				}
			},
			Expected: Configuration{
				ASNDatabase: []string{"something"},
				GeoDatabase: []string{"something else"},
			},
		}, {
			Description: "both country-database, geoip-database",
			Initial:     func() any { return Configuration{} },
			Configuration: func() any {
				return gin.H{
					"asn-database":     []string{"something"},
					"geo-database":     []string{"something else"},
					"country-database": []string{"another value"},
				}
			},
			Error: true,
		},
	})
}
