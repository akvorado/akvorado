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
			Initial:       func() interface{} { return Configuration{} },
			Configuration: func() interface{} { return nil },
			Expected:      Configuration{},
		}, {
			Description:   "empty",
			Initial:       func() interface{} { return Configuration{} },
			Configuration: func() interface{} { return gin.H{} },
			Expected:      Configuration{},
		}, {
			Description: "no country-database, no geoip-database",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-database": "something",
					"optional":     true,
				}
			},
			Expected: Configuration{
				ASNDatabase: "something",
				Optional:    true,
			},
		}, {
			Description: "country-database, no geoip-database",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-database":     "something",
					"country-database": "something else",
				}
			},
			Expected: Configuration{
				ASNDatabase: "something",
				GeoDatabase: "something else",
			},
		}, {
			Description: "no country-database, geoip-database",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-database": "something",
					"geo-database": "something else",
				}
			},
			Expected: Configuration{
				ASNDatabase: "something",
				GeoDatabase: "something else",
			},
		}, {
			Description: "both country-database, geoip-database",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-database":     "something",
					"geo-database":     "something else",
					"country-database": "another value",
				}
			},
			Error: true,
		},
	})
}
