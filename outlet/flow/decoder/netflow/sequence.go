// SPDX-FileCopyrightText: 2026 ioplane
// SPDX-License-Identifier: AGPL-3.0-only

package netflow

import "sync"

// sequenceTracker follows the sequence number of each observation domain of
// each exporter to count missing export packets (NetFlow v9) and data records
// (IPFIX). It only gives exact results when the packets of an exporter are
// processed in order, which is the case with the by-exporter load-balancing
// of the inlet.
type sequenceTracker struct {
	lock    sync.Mutex
	domains map[sequenceKey]sequenceState
}

// sequenceKey is the key structure to access the sequence state of an
// observation domain.
type sequenceKey struct {
	exporter    string
	version     uint16
	obsDomainID uint32
}

// sequenceState is the expected next sequence number of an observation domain.
type sequenceState struct {
	next  uint32
	known bool // false when the increment of the last message is unknown
}

// observe records a sequence number and returns the number of missing packets
// or records since the previous one, and whether the sequence went backward.
// For NetFlow v9, the sequence number counts export packets (RFC 3954, section
// 5.1). For IPFIX, it counts data records (RFC 7011, section 3.1), including
// those of options templates. When some sets could not be decoded, the number
// of records is unknown and the next message only resynchronizes. A step back
// is a reordered packet or an exporter restart: the state is resynchronized.
func (st *sequenceTracker) observe(key sequenceKey, seq, increment uint32, incrementKnown bool) (missing uint32, backward bool) {
	st.lock.Lock()
	defer st.lock.Unlock()
	if st.domains == nil {
		st.domains = make(map[sequenceKey]sequenceState)
	}
	previous, ok := st.domains[key]
	st.domains[key] = sequenceState{next: seq + increment, known: incrementKnown}
	if !ok || !previous.known {
		return 0, false
	}
	delta := seq - previous.next // modulo 2^32
	if delta >= 1<<31 {
		return 0, true
	}
	return delta, false
}
