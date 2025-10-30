// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers_test

import (
	"testing"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestTLSConfigurationMigration(t *testing.T) {
	helpers.TestConfigurationDecode(t, helpers.ConfigurationDecodeCases{
		{
			Description: "new skip-verify field",
			Initial:     func() any { return helpers.TLSConfiguration{} },
			Configuration: func() any {
				return gin.H{
					"enable":      true,
					"skip-verify": true,
				}
			},
			Expected: helpers.TLSConfiguration{
				Enable:     true,
				SkipVerify: true,
			},
		}, {
			Description: "no verify/skip-verify",
			Initial:     func() any { return helpers.TLSConfiguration{} },
			Configuration: func() any {
				return gin.H{
					"enable": true,
				}
			},
			Expected: helpers.TLSConfiguration{
				Enable:     true,
				SkipVerify: false,
			},
		}, {
			Description: "old verify=true migrates to skip-verify=false",
			Initial:     func() any { return helpers.TLSConfiguration{} },
			Configuration: func() any {
				return gin.H{
					"enable": true,
					"verify": true,
				}
			},
			Expected: helpers.TLSConfiguration{
				Enable:     true,
				SkipVerify: false,
			},
		}, {
			Description: "old verify=false migrates to skip-verify=true",
			Initial:     func() any { return helpers.TLSConfiguration{} },
			Configuration: func() any {
				return gin.H{
					"enable": true,
					"verify": false,
				}
			},
			Expected: helpers.TLSConfiguration{
				Enable:     true,
				SkipVerify: true,
			},
		}, {
			Description: "both verify and skip-verify causes error",
			Initial:     func() any { return helpers.TLSConfiguration{} },
			Configuration: func() any {
				return gin.H{
					"enable":      true,
					"verify":      true,
					"skip-verify": false,
				}
			},
			Error: true,
		},
	})
}
