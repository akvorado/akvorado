// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package udp

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	oobLength        = syscall.CmsgLen(4) + syscall.CmsgLen(16) // uint32 + 2*int64
	udpSocketOptions = []socketOption{
		{
			// Allow multiple listeners to bind to the same IP
			Name:      "SO_REUSEADDR",
			Level:     unix.SOL_SOCKET,
			Option:    unix.SO_REUSEADDR,
			Mandatory: true,
		}, {
			// Allow multiple listeners to bind to the same port
			Name:      "SO_REUSEPORT",
			Level:     unix.SOL_SOCKET,
			Option:    unix.SO_REUSEPORT,
			Mandatory: true,
		}, {
			// Get the number of dropped packets
			Name:   "SO_RXQ_OVFL",
			Level:  unix.SOL_SOCKET,
			Option: unix.SO_RXQ_OVFL,
		}, {
			// Ask the kernel to timestamp incoming packets
			Name:   "SO_TIMESTAMP_NEW",
			Level:  unix.SOL_SOCKET,
			Option: unix.SO_TIMESTAMP_NEW | unix.SOF_TIMESTAMPING_RX_SOFTWARE,
		},
	}
)

// parseSocketControlMessage parses b and extract the number of drops
// returned (SO_RXQ_OVFL).
func parseSocketControlMessage(b []byte) (oobMessage, error) {
	result := oobMessage{}

	cmsgs, err := syscall.ParseSocketControlMessage(b)
	if err != nil {
		return result, err
	}

	for _, cmsg := range cmsgs {
		// We know that cmsg.Data is correctly aligned for the data it contains, so we can cast it.
		if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SO_RXQ_OVFL {
			result.Drops = *(*uint32)(unsafe.Pointer(&cmsg.Data[0]))
		} else if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SO_TIMESTAMP_NEW {
			// We only are interested in the current second.
			result.Received = time.Unix(*(*int64)(unsafe.Pointer(&cmsg.Data[0])), 0)
		}
	}
	return result, nil
}
