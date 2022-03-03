package flow

// Configuration describes the configuration for the flow component
type Configuration struct {
	// Netflow defines the default listening string for netflow.
	Netflow string
	// Workers define the number of workers to use for decoding.
	Workers int
}

// DefaultConfiguration represents the default configuration for the flow component
var DefaultConfiguration = Configuration{
	Netflow: "localhost:2055",
	Workers: 1,
}
