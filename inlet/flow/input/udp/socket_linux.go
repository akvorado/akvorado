// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package udp

import (
	"syscall"
	"time"

	"akvorado/common/helpers"

	"golang.org/x/sys/unix"
)

var (
	oobLength        = syscall.CmsgLen(4) + syscall.CmsgLen(16) // uint32 + 2*int64
	udpSocketOptions = []int{
		// Allow multiple listeners to bind to the same IP/port
		unix.SO_REUSEADDR, unix.SO_REUSEPORT,
		// Get the number of dropped packets
		unix.SO_RXQ_OVFL,
		// Ask the kernel to timestamp incoming packets
		unix.SO_TIMESTAMP | unix.SOF_TIMESTAMPING_RX_SOFTWARE,
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
		if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SO_RXQ_OVFL {
			result.Drops = helpers.NativeEndian.Uint32(cmsg.Data)
		} else if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SO_TIMESTAMP {
			// We only are interested in the current second.
			result.Received = time.Unix(
				int64(helpers.NativeEndian.Uint64(cmsg.Data)),
				int64(helpers.NativeEndian.Uint64(cmsg.Data[8:]))*1000)
		}
	}
	return result, nil
}
