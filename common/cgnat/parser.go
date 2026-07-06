// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cgnat

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"time"
)

var linePattern = regexp.MustCompile(`NAT:(\d{14})\s+\S+\s+PortBatchV2(Allocated|Freed):\s+\[([^\]]+)\]`)

// ParseSyslogLine extracts a CGNAT mapping event from one syslog line.
func ParseSyslogLine(line string) (Event, error) {
	matches := linePattern.FindStringSubmatch(line)
	if len(matches) != 4 {
		return Event{}, errors.New("line does not match CGNAT pattern")
	}

	timestamp, err := time.ParseInLocation("20060102150405", matches[1], time.UTC)
	if err != nil {
		return Event{}, fmt.Errorf("invalid NAT timestamp: %w", err)
	}

	portsAndIPsPattern := regexp.MustCompile(`\s+`)
	fields := portsAndIPsPattern.Split(matches[3], -1)
	if len(fields) != 4 {
		return Event{}, fmt.Errorf("invalid mapping tuple %q", matches[3])
	}

	privateIP, err := netip.ParseAddr(fields[0])
	if err != nil {
		return Event{}, fmt.Errorf("invalid private IP %q: %w", fields[0], err)
	}
	publicIP, err := netip.ParseAddr(fields[1])
	if err != nil {
		return Event{}, fmt.Errorf("invalid public IP %q: %w", fields[1], err)
	}
	start, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return Event{}, fmt.Errorf("invalid start port %q: %w", fields[2], err)
	}
	end, err := strconv.ParseUint(fields[3], 10, 16)
	if err != nil {
		return Event{}, fmt.Errorf("invalid end port %q: %w", fields[3], err)
	}
	if start > end {
		return Event{}, fmt.Errorf("invalid port range %d-%d", start, end)
	}

	operation := OperationAllocate
	if matches[2] == "Freed" {
		operation = OperationFree
	}

	return Event{
		Timestamp: timestamp,
		Operation: operation,
		PrivateIP: privateIP,
		PublicIP:  publicIP,
		PortStart: uint16(start),
		PortEnd:   uint16(end),
	}, nil
}
