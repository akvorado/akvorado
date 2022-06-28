// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"net"
	"syscall"

	"golang.org/x/sys/unix"

	"akvorado/common/helpers"
)

var (
	oobLength = syscall.CmsgLen(4)
	// listenConfig configures a listening socket to reuse port and return overflows
	listenConfig = net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				opts := []int{unix.SO_REUSEADDR, unix.SO_REUSEPORT, unix.SO_RXQ_OVFL}
				for _, opt := range opts {
					err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, opt, 1)
					if err != nil {
						return
					}
				}
			})
			return err
		},
	}
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
