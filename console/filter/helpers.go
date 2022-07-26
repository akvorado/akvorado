// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"encoding/binary"
	"net"
)

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
