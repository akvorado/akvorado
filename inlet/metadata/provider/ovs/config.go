package ovs

import "akvorado/inlet/metadata/provider"

// Configuration describes how to connect to an OVSDB instance.
type Configuration struct {
	// Socket is the path to the OVSDB unix socket.
	Socket string `validate:"omitempty"`
	// Address is the IP address of the OVSDB server when using TCP.
	Address string `validate:"omitempty"`
	// Port is the TCP port of the OVSDB server.
	Port uint16 `validate:"min=1"`
}

// DefaultConfiguration returns default settings for the OVS provider.
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		Socket:  "/var/run/openvswitch/db.sock",
		Address: "127.0.0.1",
		Port:    6640,
	}
}
