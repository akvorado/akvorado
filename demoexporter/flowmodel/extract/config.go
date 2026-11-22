// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package extract builds training data for the flow model from a ClickHouse
// database. It only reads what the model is allowed to learn. Local prefixes are
// anonymized before entering the model.
package extract

import (
	"errors"
	"net/netip"
	"time"

	"akvorado/common/helpers"
	"akvorado/demoexporter/flowmodel"
)

// Configuration describes how to train a flow model from ClickHouse.
type Configuration struct {
	// Start is the beginning of the time range to learn from.
	Start time.Time `validate:"required"`
	// End is the end of the time range to learn from.
	End time.Time `validate:"required,gtfield=Start"`
	// Fraction is the share of the flows used for training. Keeping it well
	// below 1 hides the absolute traffic volumes.
	Fraction float64 `validate:"gt=0,lte=1"`
	// Anonymization configures how local prefixes are hidden.
	Anonymization AnonymizationConfiguration
	// ExcludeASNs lists AS numbers never learned as remote origins. List the AS
	// numbers of the network and of its group.
	ExcludeASNs []uint32
	// ExcludePrefixes lists prefixes never learned as remote prefixes. List the
	// address space of the network.
	ExcludePrefixes []netip.Prefix
	// MinCount is the minimum number of flows, after sampling, for a prefix, a
	// protocol or a port to enter the model.
	MinCount uint64 `validate:"min=1"`
	// Exporters restricts the flows to the listed exporter names. Empty means
	// all exporters.
	Exporters []string
	// MaxFlows caps the number of flows read for training.
	MaxFlows int `validate:"min=1000"`
	// Model configures the size of the model.
	Model flowmodel.Hyperparameters
	// Train configures the training loop.
	Train flowmodel.TrainOptions
}

// AnonymizationConfiguration describes how local prefixes are anonymized.
type AnonymizationConfiguration struct {
	// IPv4Prefix receives the anonymized IPv4 prefixes, for example a /8.
	IPv4Prefix netip.Prefix
	// IPv6Prefix receives the anonymized IPv6 prefixes.
	IPv6Prefix netip.Prefix
}

// DefaultConfiguration returns the default configuration for training a flow
// model.
func DefaultConfiguration() Configuration {
	return Configuration{
		Fraction: 0.1,
		MinCount: 100,
		MaxFlows: 2_000_000,
		Model:    flowmodel.DefaultHyperparameters(),
		Train:    flowmodel.DefaultTrainOptions(),
	}
}

// Check normalizes the prefixes and validates the parts that cannot be
// expressed with validation tags. It runs before any query, so a bad
// configuration is reported without touching the database.
func (c *Configuration) Check() error {
	for i, prefix := range c.ExcludePrefixes {
		c.ExcludePrefixes[i] = helpers.UnmapPrefix(prefix)
	}
	ipv4, ipv6 := c.Anonymization.IPv4Prefix, c.Anonymization.IPv6Prefix
	if !ipv4.IsValid() || !ipv4.Addr().Is4() {
		return errors.New("anonymization requires an IPv4 target prefix")
	}
	if !ipv6.IsValid() || !ipv6.Addr().Is6() || ipv6.Addr().Is4In6() {
		return errors.New("anonymization requires an IPv6 target prefix")
	}
	return nil
}
