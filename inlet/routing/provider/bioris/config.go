// SPDX-License-Identifier: AGPL-3.0-only

// Package bioris provides an interface to hydrate flow from biorouting ris
package bioris

import (
	"time"

	"akvorado/inlet/routing/provider"
)

// Configuration describes the configuration for the BioRIS component.
type Configuration struct {
	// RISInstances holds the different ris connections
	RISInstances []RISInstance `validate:"dive"`
	// Timeout defines the timeout to retrieve a result from ris connections
	Timeout time.Duration `validate:"min=1ms"`
	// Refresh defines the interval to refresh router list from RIS instances
	Refresh time.Duration `validate:"min=1s"`
	// RefreshTimeout defines the timeout to retrieve the list of routers from one RIS instance
	RefreshTimeout time.Duration `validate:"min=1s"`
}

// RISInstance stores the connection details of a single RIS connection
type RISInstance struct {
	GRPCAddr   string `validate:"required,listen"`
	GRPCSecure bool
	VRFId      uint64 `validate:"excluded_with=vrf"`
	VRF        string `validate:"excluded_with=vrfid"`
}

// DefaultConfiguration represents the default configuration for the
// RISInstance component. Without connection, the component won't report
// anything.
func DefaultConfiguration() provider.Configuration {
	return Configuration{
		RISInstances:   []RISInstance{},
		Timeout:        200 * time.Millisecond,
		Refresh:        30 * time.Minute,
		RefreshTimeout: 10 * time.Second,
	}
}
