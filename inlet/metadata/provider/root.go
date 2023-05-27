// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package provider defines the interface of a provider module for metadata.
package provider

import (
	"context"
	"net/netip"

	"akvorado/common/reporter"
)

// Interface contains the information about an interface.
type Interface struct {
	Name        string
	Description string
	Speed       uint
}

// Query is the query sent to a provider.
type Query struct {
	ExporterIP netip.Addr
	IfIndex    uint
}

// BatchQuery is a batched query.
type BatchQuery struct {
	ExporterIP netip.Addr
	IfIndexes  []uint
}

// Answer is the answer received from a provider.
type Answer struct {
	ExporterName string
	Interface
}

// Update is an update received from a provider.
type Update struct {
	Query
	Answer
}

// Provider is the interface a provider should implement.
type Provider interface {
	// Query asks the provider to query metadata for several requests. The
	// updates will be returned by calling the provided callback for each one.
	Query(ctx context.Context, query BatchQuery, put func(Update)) error
}

// Configuration defines an interface to configure a provider.
type Configuration interface {
	// New instantiates a new provider from its configuration.
	New(r *reporter.Reporter) (Provider, error)
}