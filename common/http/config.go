package http

// Configuration describes the configuration for the HTTP server.
type Configuration struct {
	// Listen defines the listening string to listen to.
	Listen string
	// Profiler enables Go profiler as /debug
	Profiler bool
}

// DefaultConfiguration is the default configuration of the HTTP server.
var DefaultConfiguration = Configuration{
	Listen: "localhost:8080",
}
