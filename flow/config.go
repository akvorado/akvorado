package flow

// Configuration describes the configuration for the flow component
type Configuration struct {
	// Listen defines the default listening string for netflow.
	Listen string
	// Workers define the number of workers to use for decoding.
	Workers int
	// QueueSize defines the size of the channel used to
	// communicate incoming flows. 0 can be used to disable
	// buffering.
	QueueSize uint
}

// DefaultConfiguration represents the default configuration for the flow component
var DefaultConfiguration = Configuration{
	Listen:    "localhost:2055",
	Workers:   1,
	QueueSize: 1000,
}
