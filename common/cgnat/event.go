// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cgnat provides shared types and parser helpers for CGNAT mapping events.
package cgnat

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"time"
)

// Operation identifies the type of CGNAT mapping event.
type Operation uint8

const (
	// OperationAllocate identifies a mapping allocation event.
	OperationAllocate Operation = iota + 1
	// OperationFree identifies a mapping deallocation event.
	OperationFree
)

// Event is one CGNAT mapping update.
type Event struct {
	Timestamp time.Time
	Operation Operation
	PrivateIP netip.Addr
	PublicIP  netip.Addr
	PortStart uint16
	PortEnd   uint16
}

type wireEvent struct {
	Timestamp int64  `json:"timestamp"`
	Operation string `json:"operation"`
	PrivateIP string `json:"private_ip"`
	PublicIP  string `json:"public_ip"`
	PortStart uint16 `json:"port_start"`
	PortEnd   uint16 `json:"port_end"`
}

func (o Operation) marshalText() (string, error) {
	switch o {
	case OperationAllocate:
		return "allocate", nil
	case OperationFree:
		return "free", nil
	default:
		return "", fmt.Errorf("unknown operation %d", o)
	}
}

func unmarshalOperation(op string) (Operation, error) {
	switch op {
	case "allocate":
		return OperationAllocate, nil
	case "free":
		return OperationFree, nil
	default:
		return 0, fmt.Errorf("unknown operation %q", op)
	}
}

// Encode serializes a CGNAT event for transport in RawFlow payload.
func Encode(event Event) ([]byte, error) {
	op, err := event.Operation.marshalText()
	if err != nil {
		return nil, err
	}
	w := wireEvent{
		Timestamp: event.Timestamp.Unix(),
		Operation: op,
		PrivateIP: event.PrivateIP.String(),
		PublicIP:  event.PublicIP.String(),
		PortStart: event.PortStart,
		PortEnd:   event.PortEnd,
	}
	return json.Marshal(w)
}

// Decode deserializes a CGNAT event from RawFlow payload.
func Decode(payload []byte) (Event, error) {
	var w wireEvent
	if err := json.Unmarshal(payload, &w); err != nil {
		return Event{}, fmt.Errorf("cannot decode payload: %w", err)
	}

	op, err := unmarshalOperation(w.Operation)
	if err != nil {
		return Event{}, err
	}
	privateIP, err := netip.ParseAddr(w.PrivateIP)
	if err != nil {
		return Event{}, fmt.Errorf("invalid private IP %q: %w", w.PrivateIP, err)
	}
	publicIP, err := netip.ParseAddr(w.PublicIP)
	if err != nil {
		return Event{}, fmt.Errorf("invalid public IP %q: %w", w.PublicIP, err)
	}
	if w.PortStart > w.PortEnd {
		return Event{}, fmt.Errorf("invalid range %d-%d", w.PortStart, w.PortEnd)
	}

	return Event{
		Timestamp: time.Unix(w.Timestamp, 0).UTC(),
		Operation: op,
		PrivateIP: privateIP,
		PublicIP:  publicIP,
		PortStart: w.PortStart,
		PortEnd:   w.PortEnd,
	}, nil
}
