// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"encoding/binary"
	"net"
	"strings"
)

// Meta is used to inject/retrieve state from the parser.
type Meta struct {
	// ReverseDirection tells if we require the reverse direction for the provided filter (used as input)
	ReverseDirection bool
	// MainTableRequired tells if the main table is required to execute the expression (used as output)
	MainTableRequired bool
}

// ReverseColumnDirection reverts the direction of a provided column name.
func ReverseColumnDirection(name string) string {
	if strings.HasPrefix(name, "Src") {
		return "Dst" + name[3:]
	}
	if strings.HasPrefix(name, "Dst") {
		return "Src" + name[3:]
	}
	if strings.HasPrefix(name, "In") {
		return "Out" + name[2:]
	}
	if strings.HasPrefix(name, "Out") {
		return "In" + name[3:]
	}
	return name
}

func (c *current) reverseColumnDirection(name string) string {
	if c.globalStore["meta"].(*Meta).ReverseDirection {
		return ReverseColumnDirection(name)
	}
	return name
}

func lastIP(subnet *net.IPNet) net.IP {
	if subnet.IP.To4() != nil {
		// IPv4 case
		ip := make(net.IP, len(subnet.IP.To4()))
		binary.BigEndian.PutUint32(ip,
			binary.BigEndian.Uint32(subnet.IP.To4())|^binary.BigEndian.Uint32(net.IP(subnet.Mask).To4()))
		return ip
	}
	// IPv6 case
	ip := make(net.IP, len(subnet.IP))
	copy(ip, subnet.IP)
	for i := range subnet.Mask {
		ip[i] = ip[i] | ^subnet.Mask[i]
	}
	return ip
}

func quote(v interface{}) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(toString(v)) + "'"
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	return v.([]interface{})
}

func toString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		panic("not a string")
	}
}
