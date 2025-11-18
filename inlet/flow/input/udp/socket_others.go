// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package udp

import (
	"errors"
)

var (
	oobLength        = 0
	udpSocketOptions = commonUDPSocketOptions
)

// parseSocketControlMessage always returns 0.
func parseSocketControlMessage(_ []byte) (oobMessage, error) {
	return oobMessage{}, nil
}

// setupReuseportEBPF is a no-op on non-Linux platforms
func setupReuseportEBPF([]uintptr) error {
	return errors.New("eBPF-controlled reuseport not supported by this platform")
}

// cleanupReuseportEBPF is a no-op on non-Linux platforms
func cleanupReuseportEBPF() {
}
