package udp

// Configuration describes UDP input configuration.
type Configuration struct {
	// Listen tells which port to listen to.
	Listen string
	// Workers define the number of workers to use for receiving flows.
	Workers int
	// QueueSize defines the size of the channel used to
	// communicate incoming flows. 0 can be used to disable
	// buffering.
	QueueSize uint
}

// DefaultConfiguration is the default configuration for this input
var DefaultConfiguration = Configuration{
	Workers:   1,
	QueueSize: 100000,
}
