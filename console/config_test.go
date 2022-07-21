// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"testing"

	"github.com/gin-gonic/gin"

	"akvorado/common/helpers"
)

func TestConfigHandler(t *testing.T) {
	config := DefaultConfiguration()
	config.Version = "1.2.3"
	_, h, _, _ := NewMock(t, config)
	helpers.TestHTTPEndpoints(t, h.Address, helpers.HTTPEndpointCases{
		{
			URL: "/api/v0/console/configuration",
			JSONOutput: gin.H{
				"version": "1.2.3",
				"defaultVisualizeOptions": gin.H{
					"start":      "6 hours ago",
					"end":        "now",
					"filter":     "InIfBoundary = external",
					"dimensions": []string{"SrcAS"},
				},
			},
		},
	})
}
