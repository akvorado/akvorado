// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package udp

var (
	oobLength        = 0
	udpSocketOptions = commonUDPSocketOptions
)

// parseSocketControlMessage always returns 0.
func parseSocketControlMessage(_ []byte) (oobMessage, error) {
	return oobMessage{}, nil
}
