// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package udp

import (
	"syscall"

	"akvorado/common/helpers"

	"golang.org/x/sys/unix"
)

var (
	oobLength        = syscall.CmsgLen(4)
	udpSocketOptions = []int{unix.SO_REUSEADDR, unix.SO_REUSEPORT, unix.SO_RXQ_OVFL}
)

// parseSocketControlMessage parses b and extract the number of drops
// returned (SO_RXQ_OVFL).
func parseSocketControlMessage(b []byte) (uint32, error) {
	cmsgs, err := syscall.ParseSocketControlMessage(b)
	if err != nil {
		return 0, err
	}
	for _, cmsg := range cmsgs {
		if cmsg.Header.Level == unix.SOL_SOCKET && cmsg.Header.Type == unix.SO_RXQ_OVFL {
			return helpers.NativeEndian.Uint32(cmsg.Data), nil
		}
	}
	return 0, nil
}
