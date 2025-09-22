// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package udp

import "golang.org/x/sys/unix"

var (
	oobLength        = 0
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
		},
	}
)

// parseSocketControlMessage always returns 0.
func parseSocketControlMessage(_ []byte) (oobMessage, error) {
	return oobMessage{}, nil
}
