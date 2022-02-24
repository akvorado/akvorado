package http

// Configuration describes the configuration for the HTTP server.
type Configuration struct {
	// Listen defines the listening string to listen to.
	Listen string
}

// DefaultConfiguration represents the default configuration for the HTTP server.
var DefaultConfiguration = Configuration{
	Listen: "localhost:8080",
}
