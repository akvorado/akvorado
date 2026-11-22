// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"net/netip"
)

// Metadata describes the vocabularies and side information of a model. It
// should not contain anything sensitive: no exporter names, no interface names,
// no AS paths, and customer prefixes are already anonymized when the metadata
// is built.
type Metadata struct {
	// Hyper holds the dimensions the model was built with.
	Hyper Hyperparameters
	// ProtoEtypes is the vocabulary of protocol and ethernet type pairs.
	ProtoEtypes []ProtoEtype
	// RemotePrefixes is the vocabulary of remote prefixes. Entries with a
	// zero-length prefix are catch-all entries for one address family.
	RemotePrefixes []RemotePrefixInfo
	// LocalPrefixes is the vocabulary of local prefixes, anonymized when they
	// hold customers. Entries with a zero-length prefix are catch-all entries
	// for one address family.
	LocalPrefixes []LocalPrefixInfo
	// Ports is the vocabulary shared by source and destination ports.
	Ports []PortRange
	// RateProfile is the relative number of flows for each hour of the week.
	// The average is 1.
	RateProfile [hourOfWeekCount]float32
}

// ProtoEtype is a pair of ethernet type and IP protocol.
type ProtoEtype struct {
	EType uint16
	Proto uint8
}

// RemotePrefixInfo describes a remote prefix. Only the public origin AS number
// is kept.
type RemotePrefixInfo struct {
	Prefix netip.Prefix
	ASN    uint32
}

// LocalPrefixInfo describes a local prefix.
type LocalPrefixInfo struct {
	Prefix netip.Prefix
	// HostBits is the log2 of the number of distinct hosts to simulate
	// in this prefix. Zero means any host.
	HostBits uint8
}

// PortRange is an entry of the port vocabulary. An exact port has
// Start equal to End.
type PortRange struct {
	Start uint16
	End   uint16
}

// vocabSizes returns the vocabulary size of each field.
func (meta *Metadata) vocabSizes() [FieldCount]int {
	return [FieldCount]int{
		FieldHourOfWeek:    hourOfWeekCount,
		FieldDirection:     directionCount,
		FieldLocalBoundary: boundaryCount,
		FieldProtoEtype:    len(meta.ProtoEtypes),
		FieldRemotePrefix:  len(meta.RemotePrefixes),
		FieldLocalPrefix:   len(meta.LocalPrefixes),
		FieldSrcPort:       len(meta.Ports),
		FieldDstPort:       len(meta.Ports),
		FieldSizeBucket:    SizeBucketCount,
		FieldPacketBucket:  PacketBucketCount,
	}
}
