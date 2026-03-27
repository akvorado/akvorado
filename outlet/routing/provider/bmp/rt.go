// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"encoding/binary"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

// RT defines a route target.
type RT uint64

// UnmarshalText parses a route target. This is parsed the same as a RD.
func (rt *RT) UnmarshalText(input []byte) error {
	var rd RD
	if err := rd.UnmarshalText(input); err != nil {
		return err
	}
	*rt = RT(rd)
	return nil
}

// MarshalText turns a route target into a textual representation.
func (rt RT) MarshalText() ([]byte, error) {
	return []byte(rt.String()), nil
}

// String turns a route target into a textual representation.
func (rt RT) String() string {
	return RD(rt).String()
}

// RTFromExtendedCommunity converts an extended community to an RT if
// it is a route target.
func RTFromExtendedCommunity(ec bgp.ExtendedCommunityInterface) (RT, bool) {
	_, subType := ec.GetTypes()
	if subType != bgp.EC_SUBTYPE_ROUTE_TARGET {
		return 0, false
	}
	buf, err := ec.Serialize()
	if err != nil || len(buf) != 8 {
		return 0, false
	}
	// Normalize to the same encoding used by RD: type in bits
	// 48-49, value in bits 0-47. The type is extracted from the
	// high byte of the extended community (masking transitive
	// bit).
	typ := buf[0] & 0x03
	var encoded [8]byte
	encoded[1] = typ
	copy(encoded[2:], buf[2:])
	return RT(binary.BigEndian.Uint64(encoded[:])), true
}
