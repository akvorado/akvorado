package cmd

import (
	"fmt"

	"akvorado/common/http"
	"akvorado/common/reporter"
)

// addCommonHTTPHandlers configures various endpoints common to all
// services. Each endpoint is registered under `/api/v0` and
// `/api/v0/SERVICE` namespaces.
func addCommonHTTPHandlers(r *reporter.Reporter, service string, httpComponent *http.Component) {
	httpComponent.AddHandler(fmt.Sprintf("/api/v0/%s/metrics", service), r.MetricsHTTPHandler())
	httpComponent.AddHandler("/api/v0/metrics", r.MetricsHTTPHandler())
	httpComponent.AddHandler(fmt.Sprintf("/api/v0/%s/healthcheck", service), r.HealthcheckHTTPHandler())
	httpComponent.AddHandler("/api/v0/healthcheck", r.HealthcheckHTTPHandler())
	httpComponent.AddHandler(fmt.Sprintf("/api/v0/%s/version", service), versionHandler())
	httpComponent.AddHandler("/api/v0/version", versionHandler())
}
