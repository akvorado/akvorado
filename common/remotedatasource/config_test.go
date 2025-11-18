// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasource

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestSourceDecode(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "Empty",
			Initial:     func() any { return Source{} },
			Configuration: func() any {
				return gin.H{
					"url":      "https://example.net",
					"interval": "10m",
				}
			},
			Expected: Source{
				URL:      "https://example.net",
				Method:   "GET",
				Timeout:  time.Minute,
				Interval: 10 * time.Minute,
				TLS: helpers.TLSConfiguration{
					SkipVerify: false,
				},
			},
		}, {
			Description: "Simple transform",
			Initial:     func() any { return Source{} },
			Configuration: func() any {
				return gin.H{
					"url":       "https://example.net",
					"interval":  "10m",
					"transform": ".[]",
				}
			},
			Expected: Source{
				URL:       "https://example.net",
				Method:    "GET",
				Timeout:   time.Minute,
				Interval:  10 * time.Minute,
				Transform: MustParseTransformQuery(".[]"),
				TLS: helpers.TLSConfiguration{
					SkipVerify: false,
				},
			},
		}, {
			Description: "Use POST",
			Initial:     func() any { return Source{} },
			Configuration: func() any {
				return gin.H{
					"url":       "https://example.net",
					"method":    "POST",
					"timeout":   "2m",
					"interval":  "10m",
					"transform": ".[]",
				}
			},
			Expected: Source{
				URL:       "https://example.net",
				Method:    "POST",
				Timeout:   2 * time.Minute,
				Interval:  10 * time.Minute,
				Transform: MustParseTransformQuery(".[]"),
				TLS: helpers.TLSConfiguration{
					SkipVerify: false,
				},
			},
		}, {
			Description: "With TLS configuration",
			Initial:     func() any { return Source{} },
			Configuration: func() any {
				return gin.H{
					"url":      "https://example.net",
					"interval": "10m",
					"tls": gin.H{
						"enable":  true,
						"ca-file": "something.crt",
					},
				}
			},
			Expected: Source{
				URL:      "https://example.net",
				Method:   "GET",
				Timeout:  time.Minute,
				Interval: 10 * time.Minute,
				TLS: helpers.TLSConfiguration{
					Enable:     true,
					SkipVerify: false,
					CAFile:     "something.crt",
				},
			},
		}, {
			Description: "Complex transform",
			Initial:     func() any { return Source{} },
			Configuration: func() any {
				return gin.H{
					"url":      "https://example.net",
					"interval": "10m",
					"transform": `
.prefixes[] | {prefix: .ip_prefix, tenant: "amazon", region: .region, role: .service|ascii_downcase}
`,
				}
			},
			Expected: Source{
				URL:      "https://example.net",
				Method:   "GET",
				Timeout:  time.Minute,
				Interval: 10 * time.Minute,
				Transform: MustParseTransformQuery(`
.prefixes[] | {prefix: .ip_prefix, tenant: "amazon", region: .region, role: .service|ascii_downcase}
`),
				TLS: helpers.TLSConfiguration{
					SkipVerify: false,
				},
			},
		}, {
			Description: "Incorrect transform",
			Initial:     func() any { return Source{} },
			Configuration: func() any {
				return gin.H{
					"url":       "https://example.net",
					"interval":  "10m",
					"transform": "878778&&",
				}
			},
			Error: true,
		},
	})
}
