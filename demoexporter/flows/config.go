// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flows

import (
	"net/netip"
	"time"
)

// Configuration describes the configuration for the flows component.
type Configuration struct {
	// SamplingRate defines the sampling rate for this device.
	SamplingRate int `validate:"min=1"`
	// Flows describe the flows we want to generate.
	Flows []FlowConfiguration `validate:"min=1,dive"`
	// Target specify the IP address and port to generate flows to.
	Target string `validate:"required,hostname_port"`
	// Seed defines a seed to add to the random generator. Without
	// one, all exporters will produce the same data if provided
	// the same flows.
	Seed int64
}

// FlowConfiguration describes the configuration for a flow.
type FlowConfiguration struct {
	// PerSecond defines how many of those flows should be created per second
	PerSecond float64 `validate:"required,gt=0"`
	// InIfIndex defines the source interface
	InIfIndex []int `validate:"min=1,dive,min=1"`
	// OutIfIndex defines the output interface
	OutIfIndex []int `validate:"min=1,dive,min=1"`
	// PeakHour defines the peak hour
	PeakHour time.Duration `validate:"required,min=0,max=24h"`
	// PeakMultiplier defines how to multiply the `PerSecond` when near the peak hour
	Multiplier float64 `validate:"required,gt=0"`
	// SrcNet defines the source network to use
	SrcNet netip.Prefix `validate:"required"`
	// DstNet defines the destination network to use
	DstNet netip.Prefix `validate:"required"`
	// SrcAS defines the source AS number to use
	SrcAS []uint32 `validate:"min=1"`
	// DstAS defines the destination AS number to use
	DstAS []uint32 `validate:"min=1"`
	// SrcPort defines the source port to use
	SrcPort []uint16
	// DstPort defines the destination port to use
	DstPort []uint16
	// Proto defines the IP protocol to use
	Protocol []string `validate:"min=1,dive,oneof=tcp udp icmp"`
	// Size defines the packet size to use
	Size uint `validate:"isdefault|min=64,isdefault|max=9000"`
	// ReverseDirectionRatio generate a second flow for each flow
	// generated in the opposite direction, by applying the
	// provided ratio for the Size.
	ReverseDirectionRatio float32 `validate:"min=0"`
}

// DefaultConfiguration represents the default configuration for the flows component.
func DefaultConfiguration() Configuration {
	return Configuration{
		SamplingRate: 1000,
	}
}
