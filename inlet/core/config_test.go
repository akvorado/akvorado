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
			Description:    "nil",
			Initial:        func() interface{} { return Configuration{} },
			Configuration:  func() interface{} { return nil },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description:    "empty",
			Initial:        func() interface{} { return Configuration{} },
			Configuration:  func() interface{} { return gin.H{} },
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description: "ignore-asn-from-flow = false",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"ignore-asn-from-flow": false,
				}
			},
			Expected:       Configuration{},
			SkipValidation: true,
		}, {
			Description: "ignore-asn-from-flow = true",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"ignore-asn-from-flow": true,
				}
			},
			Expected: Configuration{
				ASNProviders: []ASNProvider{ASNProviderRouting},
			},
			SkipValidation: true,
		}, {
			Description: "ignore-asn-from-flow and asn-providers",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"ignore-asn-from-flow": true,
					"asn-providers":        []string{"geoip", "flow"},
				}
			},
			Error:          true,
			SkipValidation: true,
		}, {
			Description: "asn-providers only",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-providers": []string{"flow-except-private", "routing", "flow"},
				}
			},
			Expected: Configuration{
				ASNProviders: []ASNProvider{ASNProviderFlowExceptPrivate, ASNProviderRouting, ASNProviderFlow},
			},
			SkipValidation: true,
		}, {
			Description: "net-providers with bmp",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"net-providers": []string{"flow", "bmp"},
				}
			},
			Expected: Configuration{
				NetProviders: []NetProvider{NetProviderFlow, NetProviderRouting},
			},
			SkipValidation: true,
		}, {
			Description: "asn-providers with bmp",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"asn-providers": []string{"flow", "bmp", "bmp-except-private"},
				}
			},
			Expected: Configuration{
				ASNProviders: []ASNProvider{ASNProviderFlow, ASNProviderRouting, ASNProviderRoutingExceptPrivate},
			},
			SkipValidation: true,
		}, {
			Description: "net-providers",
			Initial:     func() interface{} { return Configuration{} },
			Configuration: func() interface{} {
				return gin.H{
					"net-providers": []string{"flow", "routing"},
				}
			},
			Expected: Configuration{
				NetProviders: []NetProvider{NetProviderFlow, NetProviderRouting},
			},
			SkipValidation: true,
		},
	})
}

func TestMarshalUnmarshal(t *testing.T) {
	asnProviderMap.TestMarshalUnmarshal(t)
	netProviderMap.TestMarshalUnmarshal(t)
}
