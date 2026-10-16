// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sctp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/georgeyanev/go-sctp"

	"akvorado/common/helpers"
)

// record is a message or a notification queued in the fake socket.
type record struct {
	data         []byte
	notification bool
	more         bool // no SCTP_EOR after this record
	err          error
}

// fakeSocket returns the records like the kernel: a record larger than the
// buffer is returned in several pieces and only the last one has SCTP_EOR.
type fakeSocket struct {
	records []record
	offset  int
}

func (s *fakeSocket) read(b []byte) (int, int, error) {
	if len(s.records) == 0 {
		return 0, 0, io.EOF
	}
	rec := s.records[0]
	if rec.err != nil {
		s.records = s.records[1:]
		return 0, 0, rec.err
	}
	n := copy(b, rec.data[s.offset:])
	s.offset += n
	flags := 0
	if rec.notification {
		flags |= sctp.SCTP_NOTIFICATION
	}
	if s.offset == len(rec.data) {
		if !rec.more {
			flags |= sctp.SCTP_EOR
		}
		s.records = s.records[1:]
		s.offset = 0
	}
	return n, flags, nil
}

func partialDeliveryEvent(indication uint32) []byte {
	b := make([]byte, 24)
	binary.NativeEndian.PutUint16(b[0:2], uint16(sctp.SCTP_PARTIAL_DELIVERY_EVENT))
	binary.NativeEndian.PutUint32(b[4:8], 24)
	binary.NativeEndian.PutUint32(b[8:12], indication)
	return b
}

func assocChangeEvent() []byte {
	b := make([]byte, 20)
	binary.NativeEndian.PutUint16(b[0:2], uint16(sctp.SCTP_ASSOC_CHANGE))
	binary.NativeEndian.PutUint32(b[4:8], 20)
	return b
}

// pieces splits data in pieces of the provided sizes, without SCTP_EOR.
func pieces(data []byte, sizes ...int) []record {
	records := []record{}
	for _, size := range sizes {
		records = append(records, record{data: data[:size], more: true})
		data = data[size:]
	}
	return records
}

func TestMessageReader(t *testing.T) {
	small := bytes.Repeat([]byte("a"), 100)
	large := bytes.Repeat([]byte("b"), maxMessageSize)
	tooLong := bytes.Repeat([]byte("c"), maxMessageSize+1)
	huge := bytes.Repeat([]byte("d"), 100000)
	cases := []struct {
		Description string
		Records     []record
		Expected    [][]byte
		Error       error
		Dropped     []error
	}{
		{
			Description: "no message",
			Error:       io.EOF,
		}, {
			Description: "one message",
			Records:     []record{{data: small}},
			Expected:    [][]byte{small},
			Error:       io.EOF,
		}, {
			Description: "several messages",
			Records:     []record{{data: small}, {data: large}, {data: small}},
			Expected:    [][]byte{small, large, small},
			Error:       io.EOF,
		}, {
			Description: "message in several pieces",
			Records: append(
				pieces(large[:3000], 1000, 1000),
				record{data: large[2000:3000]}),
			Expected: [][]byte{large[:3000]},
			Error:    io.EOF,
		}, {
			Description: "notification in the middle of a message",
			Records: []record{
				{data: small[:50], more: true},
				{data: assocChangeEvent(), notification: true},
				{data: small[50:]},
			},
			Expected: [][]byte{small},
			Error:    io.EOF,
		}, {
			Description: "partial delivery aborted",
			Records: []record{
				{data: large[:1000], more: true},
				{data: partialDeliveryEvent(sctpPartialDeliveryAborted), notification: true},
				{data: small},
			},
			Expected: [][]byte{small},
			Error:    io.EOF,
			Dropped:  []error{errPartialDeliveryAborted},
		}, {
			Description: "partial delivery event between messages",
			Records: []record{
				{data: small},
				{data: partialDeliveryEvent(sctpPartialDeliveryAborted), notification: true},
				{data: small},
			},
			Expected: [][]byte{small, small},
			Error:    io.EOF,
		}, {
			Description: "other partial delivery event",
			Records: []record{
				{data: small[:50], more: true},
				{data: partialDeliveryEvent(1), notification: true},
				{data: small[50:]},
			},
			Expected: [][]byte{small},
			Error:    io.EOF,
		}, {
			Description: "message too long",
			Records:     []record{{data: small}, {data: tooLong}, {data: small}},
			Expected:    [][]byte{small, small},
			Error:       io.EOF,
			Dropped:     []error{errMessageTooLong},
		}, {
			Description: "message too long in several pieces",
			Records: append(
				pieces(huge, 40000, 30000),
				record{data: huge[70000:]},
				record{data: small}),
			Expected: [][]byte{small},
			Error:    io.EOF,
			Dropped:  []error{errMessageTooLong},
		}, {
			Description: "partial delivery aborted for a message too long",
			Records: []record{
				{data: tooLong[:maxMessageSize], more: true},
				{data: tooLong[maxMessageSize : maxMessageSize+1], more: true},
				{data: partialDeliveryEvent(sctpPartialDeliveryAborted), notification: true},
				{data: small},
			},
			Expected: [][]byte{small},
			Error:    io.EOF,
			Dropped:  []error{errMessageTooLong},
		}, {
			Description: "end in the middle of a message",
			Records:     []record{{data: small}, {data: small[:50], more: true}},
			Expected:    [][]byte{small},
			Error:       io.ErrUnexpectedEOF,
		}, {
			Description: "read error",
			Records:     []record{{data: small}, {err: errors.New("read error")}},
			Expected:    [][]byte{small},
			Error:       errors.New("read error"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			socket := &fakeSocket{records: tc.Records}
			var dropped []error
			reader := newMessageReader(socket.read, func(reason error) {
				dropped = append(dropped, reason)
			})
			var got [][]byte
			var err error
			for {
				var payload []byte
				payload, err = reader.next()
				if err != nil {
					break
				}
				got = append(got, bytes.Clone(payload))
			}
			if err == nil || err.Error() != tc.Error.Error() {
				t.Errorf("next() error:\n%+v", err)
			}
			if diff := helpers.Diff(got, tc.Expected); diff != "" {
				t.Errorf("next() (-got, +want):\n%s", diff)
			}
			if diff := helpers.Diff(dropped, tc.Dropped); diff != "" {
				t.Errorf("dropped (-got, +want):\n%s", diff)
			}
		})
	}
}
