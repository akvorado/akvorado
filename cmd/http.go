// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"fmt"

	"akvorado/common/httpserver"
	"akvorado/common/reporter"
)

// addCommonHTTPHandlers configures various endpoints common to all
// services. Each endpoint is registered under `/api/v0` and
// `/api/v0/SERVICE` namespaces.
func addCommonHTTPHandlers(r *reporter.Reporter, service string, httpComponent *httpserver.Component) {
	httpComponent.AddHandler(fmt.Sprintf("/api/v0/%s/metrics", service), r.MetricsHTTPHandler())
	httpComponent.AddHandler("/api/v0/metrics", r.MetricsHTTPHandler())
	httpComponent.GinRouter.GET(fmt.Sprintf("/api/v0/%s/healthcheck", service), r.HealthcheckHTTPHandler)
	httpComponent.GinRouter.GET("/api/v0/healthcheck", r.HealthcheckHTTPHandler)
	httpComponent.GinRouter.GET(fmt.Sprintf("/api/v0/%s/version", service), versionHandler)
	httpComponent.GinRouter.GET("/api/v0/version", versionHandler)
}
