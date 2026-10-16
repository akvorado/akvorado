// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !linux

package sctp

import "syscall"

// enablePRSCTP does nothing as SCTP is only supported on Linux.
func enablePRSCTP(syscall.RawConn) error {
	return nil
}

// subscribePartialDelivery does nothing as SCTP is only supported on Linux.
func subscribePartialDelivery(syscall.RawConn) error {
	return nil
}
