package web

// Configuration describes the configuration for the web component.
type Configuration struct {
	// GrafanaURL is the URL to acess Grafana.
	GrafanaURL string
	// ServeLiveFS serve files from the filesystem instead of the embedded versions.
	ServeLiveFS bool
}

// DefaultConfiguration represents the default configuration for the web exporter.
var DefaultConfiguration = Configuration{}
