// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flow

import (
	"akvorado/common/helpers"
	"akvorado/common/pb"
)

// Configuration describes the configuration for the flow component.
type Configuration struct {
	// Decapsulate maps encapsulating source prefixes to the decapsulation
	// protocol to apply to matching flows.
	Decapsulate *helpers.SubnetMap[pb.RawFlow_DecapsulationProtocol]
	// StatePersistFile defines a file to store decoder state (templates, sampling
	// rates) to survive restarts.
	StatePersistFile string `validate:"isdefault|filepath"`
}

// DefaultConfiguration returns the default configuration for the flow component.
func DefaultConfiguration() Configuration {
	return Configuration{}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(helpers.SubnetMapUnmarshallerHook[pb.RawFlow_DecapsulationProtocol]())
	helpers.RegisterSubnetMapValidation[pb.RawFlow_DecapsulationProtocol]()
}
