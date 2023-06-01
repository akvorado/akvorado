// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bioris

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
		},
		{
			Description:   "empty",
			Initial:       func() interface{} { return Configuration{} },
			Configuration: func() interface{} { return gin.H{} },
			Expected:      Configuration{},
		},
		{
			Description: "RisInstance",
			Initial:     func() interface{} { return RISInstance{} },
			Configuration: func() interface{} {
				return gin.H{
					"grpcaddr":   "example.com:8080",
					"grpcsecure": true,
					"vrfid":      1234,
				}
			},
			Expected: RISInstance{
				GRPCAddr:   "example.com:8080",
				GRPCSecure: true,
				VRFId:      1234,
			},
		},
	})
}
