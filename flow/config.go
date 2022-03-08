package flow

// Configuration describes the configuration for the flow component
type Configuration struct {
	// Netflow defines the default listening string for netflow.
	Netflow string
	// Workers define the number of workers to use for decoding.
	Workers int
	// BufferLength defines the length of the channel used to
	// communicate incoming flows. 0 can be used to disable
	// buffering.
	BufferLength uint
}

// DefaultConfiguration represents the default configuration for the flow component
var DefaultConfiguration = Configuration{
	Netflow:      "localhost:2055",
	Workers:      1,
	BufferLength: 1000,
}
