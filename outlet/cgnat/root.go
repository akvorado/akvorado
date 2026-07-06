// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package cgnat maintains CGNAT mapping state used to enrich flow records.
package cgnat

import (
	"fmt"
	"net/netip"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

	commoncgnat "akvorado/common/cgnat"
	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// Match is the lookup result for one flow endpoint.
type Match struct {
	PrivateIP netip.Addr
	PublicIP  netip.Addr
	PortStart uint16
	PortEnd   uint16
}

type mappingSession struct {
	privateIP netip.Addr
	publicIP  netip.Addr
	portStart uint16
	portEnd   uint16
	start     time.Time
	end       time.Time
}

// Component represents the CGNAT mapping cache.
type Component struct {
	r      *reporter.Reporter
	t      tomb.Tomb
	config Configuration

	mu       sync.RWMutex
	sessions map[netip.Addr][]mappingSession
}

// Dependencies are component dependencies.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new CGNAT mapping component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := &Component{
		r:        r,
		config:   configuration,
		sessions: make(map[netip.Addr][]mappingSession),
	}
	dependencies.Daemon.Track(&c.t, "outlet/cgnat")
	return c, nil
}

// Start starts background cleanup.
func (c *Component) Start() error {
	c.t.Go(func() error {
		ticker := time.NewTicker(c.config.CleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.t.Dying():
				return nil
			case <-ticker.C:
				cutoff := time.Now().Add(-c.config.Retention)
				c.prune(cutoff)
			}
		}
	})
	return nil
}

// Stop stops the CGNAT component.
func (c *Component) Stop() error {
	c.t.Kill(nil)
	return c.t.Wait()
}

// UpdateFromPayload updates the cache from one encoded event payload.
func (c *Component) UpdateFromPayload(payload []byte) error {
	event, err := commoncgnat.Decode(payload)
	if err != nil {
		return err
	}
	c.Update(event)
	return nil
}

// Update applies one CGNAT mapping event.
func (c *Component) Update(event commoncgnat.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sessions := c.sessions[event.PublicIP]
	if event.Operation == commoncgnat.OperationAllocate {
		sessions = append(sessions, mappingSession{
			privateIP: event.PrivateIP,
			publicIP:  event.PublicIP,
			portStart: event.PortStart,
			portEnd:   event.PortEnd,
			start:     event.Timestamp,
		})
		c.sessions[event.PublicIP] = sessions
		return
	}

	for i := len(sessions) - 1; i >= 0; i-- {
		s := &sessions[i]
		if !s.end.IsZero() {
			continue
		}
		if s.privateIP != event.PrivateIP || s.portStart != event.PortStart || s.portEnd != event.PortEnd {
			continue
		}
		s.end = event.Timestamp
		break
	}
	c.sessions[event.PublicIP] = sessions
}

// Lookup retrieves the active mapping for public endpoint at a given timestamp.
func (c *Component) Lookup(ts time.Time, publicIP netip.Addr, port uint16) (Match, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sessions := c.sessions[publicIP]
	for i := len(sessions) - 1; i >= 0; i-- {
		s := sessions[i]
		if port < s.portStart || port > s.portEnd {
			continue
		}
		if ts.Before(s.start) {
			continue
		}
		if !s.end.IsZero() && ts.After(s.end) {
			continue
		}
		return Match{
			PrivateIP: s.privateIP,
			PublicIP:  s.publicIP,
			PortStart: s.portStart,
			PortEnd:   s.portEnd,
		}, true
	}

	return Match{}, false
}

func (c *Component) prune(cutoff time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for publicIP, sessions := range c.sessions {
		kept := sessions[:0]
		for _, session := range sessions {
			if session.end.IsZero() || session.end.After(cutoff) {
				kept = append(kept, session)
			}
		}
		if len(kept) == 0 {
			delete(c.sessions, publicIP)
			continue
		}
		c.sessions[publicIP] = kept
	}
}

// Stats returns basic cache counters, useful in tests.
func (c *Component) Stats() (publicIPs, sessions int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, list := range c.sessions {
		sessions += len(list)
	}
	return len(c.sessions), sessions
}

func (m Match) String() string {
	return fmt.Sprintf("%s <= %s:%d-%d", m.PrivateIP, m.PublicIP, m.PortStart, m.PortEnd)
}
