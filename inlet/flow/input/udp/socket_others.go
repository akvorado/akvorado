// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package udp

import (
	"net"

	"golang.org/x/sys/unix"
)

var (
	oobLength        = 0
	udpSocketOptions = []int{unix.SO_REUSEADDR, unix.SO_REUSEPORT}
)

// parseSocketControlMessage always returns 0.
func parseSocketControlMessage(_ []byte) (oobMessage, error) {
	return oobMessage{}, nil
}

// setReadBuffer sets the read buffer for an UDP connection.
func setReadBuffer(conn *net.UDPConn, bytes int) error {
	return conn.SetReadBuffer(bytes)
}
