// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sctp

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/georgeyanev/go-sctp"
)

const (
	// maxMessageSize is the maximum size of a message.
	maxMessageSize = 65535
	// notificationSpace is the space kept free in the buffer to receive a
	// notification in the middle of a message.
	notificationSpace = 512
	// sctpPartialDeliveryAborted is from linux/sctp.h.
	sctpPartialDeliveryAborted = 0
)

var (
	errMessageTooLong         = errors.New("message too long")
	errPartialDeliveryAborted = errors.New("partial delivery aborted")
)

// readFunc reads a piece of a message or a notification into b.
type readFunc func(b []byte) (n int, flags int, err error)

// messageReader puts back together the messages of an SCTP association. A
// message can be received in several pieces. With PR-SCTP, the sender can give
// up a message after the first pieces were received. The kernel then sends a
// partial delivery event and the pieces already received are dropped.
type messageReader struct {
	read    readFunc
	buf     []byte
	dropped func(reason error)
}

func newMessageReader(read readFunc, dropped func(reason error)) *messageReader {
	return &messageReader{
		read:    read,
		buf:     make([]byte, maxMessageSize+notificationSpace),
		dropped: dropped,
	}
}

// next returns the next complete message. The returned slice is only valid
// until the next call.
func (r *messageReader) next() ([]byte, error) {
	n := 0
	discarding := false
	for {
		m, flags, err := r.read(r.buf[n:])
		if err != nil {
			if errors.Is(err, io.EOF) && (n > 0 || discarding) {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if flags&sctp.SCTP_NOTIFICATION != 0 {
			if flags&sctp.SCTP_EOR == 0 {
				return nil, errors.New("truncated SCTP notification")
			}
			if isPartialDeliveryAborted(r.buf[n : n+m]) {
				if n > 0 {
					r.dropped(errPartialDeliveryAborted)
				}
				n = 0
				discarding = false
			}
			continue
		}
		if discarding {
			discarding = flags&sctp.SCTP_EOR == 0
			continue
		}
		n += m
		if n > maxMessageSize {
			r.dropped(errMessageTooLong)
			n = 0
			discarding = flags&sctp.SCTP_EOR == 0
			continue
		}
		if flags&sctp.SCTP_EOR != 0 {
			return r.buf[:n], nil
		}
	}
}

// isPartialDeliveryAborted tells if the notification is a partial delivery
// event telling the current message is aborted (struct sctp_pdapi_event).
func isPartialDeliveryAborted(b []byte) bool {
	if len(b) < 12 {
		return false
	}
	return binary.NativeEndian.Uint16(b[0:2]) == uint16(sctp.SCTP_PARTIAL_DELIVERY_EVENT) &&
		binary.NativeEndian.Uint32(b[8:12]) == sctpPartialDeliveryAborted
}
