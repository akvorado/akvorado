// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
)

// RD defines a route distinguisher.
type RD uint64

// UnmarshalText parses a route distinguisher.
func (rd *RD) UnmarshalText(input []byte) error {
	// We can have several formats:
	// 1. 2-byte ASN : index
	// 2. IPv4 address : index
	// 3. 4-byte ASN : index (4-byte can be in asdot format)
	// We also accept a specific type with type : X : index or just an uint64
	text := string(input)
	elems := strings.Split(text, ":")
	typ := -1
	switch len(elems) {
	case 1:
		result, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return errors.New("cannot parse RD as a 64-bit number")
		}
		*rd = RD(result)
		return nil
	case 3:
		var err error
		typ, err = strconv.Atoi(elems[0])
		if err != nil || typ < 0 || typ > 2 {
			return errors.New("cannot parse RD type")
		}
		elems = elems[1:]
		fallthrough
	case 2:
		if typ == 1 || (typ == -1 && strings.Count(elems[0], ".") > 0) {
			// IPv4 : index
			ip := net.ParseIP(elems[0])
			if ip == nil || ip.To4() == nil {
				return errors.New("cannot parse RD as IPv4 address + index")
			}
			index, err := strconv.ParseUint(elems[1], 10, 16)
			if err != nil {
				return errors.New("cannot parse RD as IPv4 address + index")
			}
			*rd = RD((1 << 48) + // Type
				(uint64(binary.BigEndian.Uint32(ip.To4())) << 16) +
				index)
			return nil
		}
		asn, err := strconv.ParseUint(elems[0], 10, 32)
		if err != nil {
			return errors.New("cannot parse RD as ASN + index")
		}
		index, err := strconv.ParseUint(elems[1], 10, 32)
		if err != nil {
			return errors.New("cannot parse RD as ASN + index")
		}
		if typ == 0 && asn > 65535 {
			return errors.New("cannot parse RD as ASN2 + index")
		} else if asn <= 65535 && typ != 2 {
			*rd = RD((0 << 48) + // Type
				(asn << 32) +
				index)
			return nil
		} else if index > 65535 {
			return errors.New("cannot parse RD as ASN4 + index")
		}
		*rd = RD((2 << 48) + // Type
			(asn << 16) +
			index)
		return nil
	default:
		return errors.New("cannot parse RD")
	}
}

// MarshalText turns a route distinguisher into a textual representation.
func (rd RD) MarshalText() ([]byte, error) {
	return []byte(rd.String()), nil
}

// String turns a route distinguisher into a textual representation.
func (rd RD) String() string {
	typ := uint64(rd) >> 48
	remaining := uint64(rd) & 0xffffffffffff
	switch typ {
	case 0:
		return fmt.Sprintf("%d:%d", (remaining>>32)&0xffff, remaining&0xffffffff)
	case 1:
		return fmt.Sprintf("%d.%d.%d.%d:%d",
			(remaining>>40)&0xff,
			(remaining>>32)&0xff,
			(remaining>>24)&0xff,
			(remaining>>16)&0xff,
			remaining&0xffff)
	case 2:
		asn := (remaining >> 16) & 0xffffffff
		if asn <= 65535 {
			return fmt.Sprintf("2:%d:%d", asn, remaining&0xffff)
		}
		return fmt.Sprintf("%d:%d", asn, remaining&0xffff)
	}
	return ""
}

const errorRD = RD(65535 << 48)

// RDFromRouteDistinguisherInterface converts a RD from GoBGP to our representation.
func RDFromRouteDistinguisherInterface(input bgp.RouteDistinguisherInterface) RD {
	buf, err := input.Serialize()
	if err != nil || len(buf) != 8 {
		return errorRD
	}
	return RD(binary.BigEndian.Uint64(buf))
}
