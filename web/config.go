package web

// Configuration describes the configuration for the web component.
type Configuration struct {
	// GrafanaURL is the URL to acess Grafana.
	GrafanaURL string
}

// DefaultConfiguration represents the default configuration for the web exporter.
var DefaultConfiguration = Configuration{}
