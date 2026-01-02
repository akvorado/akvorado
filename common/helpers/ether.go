// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import "net"

// MACToUint64 converts a MAC address to an uint64
func MACToUint64(mac net.HardwareAddr) uint64 {
	if len(mac) != 6 {
		return 0
	}
	return uint64(mac[0])<<40 |
		uint64(mac[1])<<32 |
		uint64(mac[2])<<24 |
		uint64(mac[3])<<16 |
		uint64(mac[4])<<8 |
		uint64(mac[5])
}
