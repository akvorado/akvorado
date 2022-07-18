// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type oobMessage struct {
	Drops    uint32
	Received time.Time
}

var (
	// listenConfig configures a listening socket to reuse port and return overflows
	listenConfig = net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				opts := udpSocketOptions
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
