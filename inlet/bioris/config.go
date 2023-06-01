// SPDX-License-Identifier: AGPL-3.0-only

// Package bioris provides an interface to hydrate flow from biorouting ris
package bioris

// Configuration describes the configuration for the BioRIS component.
type Configuration struct {
	// RISInstances holds the different ris connections
	RISInstances []RISInstance
}

// RISInstance stores the connection details of a single RIS connection
type RISInstance struct {
	GRPCAddr   string
	GRPCSecure bool
	VRFId      uint64
	VRF        string
}

// DefaultConfiguration represents the default configuration for the
// RISInstance component. Without connection, the component won't report
// anything.
func DefaultConfiguration() Configuration {
	return Configuration{
		RISInstances: []RISInstance{},
	}
}
