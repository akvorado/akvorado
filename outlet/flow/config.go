// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"net/netip"

	"akvorado/common/pb"
)

// Configuration describes the configuration for the flow component.
type Configuration struct {
	// StatePersistFile defines a file to store decoder state (templates, sampling
	// rates) to survive restarts.
	Decapsulation    []DecapsulationConfiguration `validate:"dive"`
	StatePersistFile string                       `validate:"isdefault|filepath"`
}

// DecapsulationConfiguration selects source and destination prefixes to decapsulate
type DecapsulationConfiguration struct {
	Protocol  pb.RawFlow_DecapsulationProtocol `validate:"required"`
	SrcPrefix netip.Prefix
}

// DefaultConfiguration returns the default configuration for the flow component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}
