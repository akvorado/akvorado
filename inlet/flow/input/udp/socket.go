// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package udp

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"akvorado/common/reporter"

	"golang.org/x/sys/unix"
)

// commonUDPSocketOptions are the options common to all OS.
var commonUDPSocketOptions = []socketOption{
	{
		// Allow a listener to bind again to a socket that was just closed
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
	},
}

type oobMessage struct {
	Drops    uint32
	Received time.Time
}

// socketOption describes a socket option to be applied.
type socketOption struct {
	Name      string
	Level     int
	Option    int
	Mandatory bool
}

// listenConfig configures a listening socket with udpSocketOptions
var listenConfig = func(r *reporter.Reporter, opts []socketOption, fds *[]uintptr) *net.ListenConfig {
	return &net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var err error
			for _, opt := range opts {
				c.Control(func(fd uintptr) {
					err = unix.SetsockoptInt(int(fd), opt.Level, opt.Option, 1)
				})
				if err != nil {
					if opt.Mandatory {
						return fmt.Errorf("cannot set option %s: %w", opt.Name, err)
					}
					r.Warn().Err(err).Msgf("cannot set option %s", opt.Name)
				}
			}

			if fds != nil {
				c.Control(func(fd uintptr) {
					*fds = append(*fds, fd)
				})
			}

			return nil
		},
	}
}
