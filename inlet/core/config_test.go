// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/gin-gonic/gin"
)

func TestDefaultConfiguration(t *testing.T) {
	if err := helpers.Validate.Struct(DefaultConfiguration()); err != nil {
		t.Fatalf("validate.Struct() error:\n%+v", err)
	}
}

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
			Description: "ignore-asn-from-flow = false",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"ignore-asn-from-flow": false,
				}
			},
			Expected: Configuration{},
		}, {
			Description: "ignore-asn-from-flow = true",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"ignore-asn-from-flow": true,
				}
			},
			Expected: Configuration{
				ASNProviders: []ASNProvider{ProviderGeoIP},
			},
		}, {
			Description: "ignore-asn-from-flow and asn-providers",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"ignore-asn-from-flow": true,
					"asn-providers":        []string{"geoip", "flow"},
				}
			},
			Error: true,
		}, {
			Description: "asn-providers only",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-providers": []string{"flow-except-private", "geoip", "flow"},
				}
			},
			Expected: Configuration{
				ASNProviders: []ASNProvider{ProviderFlowExceptPrivate, ProviderGeoIP, ProviderFlow},
			},
		},
	})
}
