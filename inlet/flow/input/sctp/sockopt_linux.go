// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package sctp

import (
	"encoding/binary"
	"syscall"

	"github.com/georgeyanev/go-sctp"
	"golang.org/x/sys/unix"
)

// From linux/sctp.h
const (
	sctpPRSupported = 113
	sctpEvent       = 127
	sctpFutureAssoc = 0
)

// enablePRSCTP enables PR-SCTP (RFC 3758) for the associations accepted by
// the socket.
func enablePRSCTP(c syscall.RawConn) error {
	// struct sctp_assoc_value
	var value [8]byte
	binary.NativeEndian.PutUint32(value[0:4], sctpFutureAssoc)
	binary.NativeEndian.PutUint32(value[4:8], 1)
	var err error
	if cerr := c.Control(func(fd uintptr) {
		err = unix.SetsockoptString(int(fd), unix.IPPROTO_SCTP, sctpPRSupported, string(value[:]))
	}); cerr != nil {
		return cerr
	}
	return err
}

// subscribePartialDelivery subscribes to the partial delivery events for the
// associations accepted by the socket.
func subscribePartialDelivery(c syscall.RawConn) error {
	// struct sctp_event
	var event [8]byte
	binary.NativeEndian.PutUint32(event[0:4], sctpFutureAssoc)
	binary.NativeEndian.PutUint16(event[4:6], uint16(sctp.SCTP_PARTIAL_DELIVERY_EVENT))
	event[6] = 1
	var err error
	if cerr := c.Control(func(fd uintptr) {
		err = unix.SetsockoptString(int(fd), unix.IPPROTO_SCTP, sctpEvent, string(event[:]))
	}); cerr != nil {
		return cerr
	}
	return err
}
