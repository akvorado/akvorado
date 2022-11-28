// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package bmp provides a BMP server to receive BGP routes from
// various exporters.
package bmp

import (
	"fmt"
	"net"
	"time"

	"github.com/benbjohnson/clock"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/helpers/sync"
	"akvorado/common/reporter"
)

// Component represents the BMP compomenent.
type Component struct {
	r           *reporter.Reporter
	d           *Dependencies
	t           tomb.Tomb
	config      Configuration
	acceptedRDs map[uint64]struct{}

	address net.Addr
	metrics metrics

	// RIB management with peers
	rib               *rib
	peers             map[peerKey]*peerInfo
	peerRemovalChan   chan peerKey
	lastPeerReference uint32
	staleTimer        *clock.Timer
	mu                sync.RWMutex
}

// Dependencies define the dependencies of the BMP component.
type Dependencies struct {
	Daemon daemon.Component
	Clock  clock.Clock
}

// New creates a new BMP component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	c := Component{
		r:      r,
		d:      &dependencies,
		config: configuration,

		rib:             newRIB(),
		peers:           make(map[peerKey]*peerInfo),
		peerRemovalChan: make(chan peerKey, configuration.PeerRemovalMaxQueue),
	}
	if len(c.config.RDs) > 0 {
		c.acceptedRDs = make(map[uint64]struct{})
		for _, rd := range c.config.RDs {
			c.acceptedRDs[uint64(rd)] = struct{}{}
		}
	}
	c.staleTimer = c.d.Clock.AfterFunc(time.Hour, c.removeStalePeers)

	c.d.Daemon.Track(&c.t, "inlet/bmp")
	c.initMetrics()
	return &c, nil
}

// Start starts the BMP component.
func (c *Component) Start() error {
	c.r.Info().Msg("starting BMP component")
	listener, err := net.Listen("tcp", c.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen to %v: %w", c.config.Listen, err)
	}
	c.address = listener.Addr()

	// Peer removal
	c.t.Go(c.peerRemovalWorker)

	// Listener
	c.t.Go(func() error {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if c.t.Alive() {
					return fmt.Errorf("cannot accept new connection: %w", err)
				}
				return nil
			}
			c.t.Go(func() error {
				return c.serveConnection(conn.(*net.TCPConn))
			})
		}
	})
	c.t.Go(func() error {
		<-c.t.Dying()
		listener.Close()
		return nil
	})
	return nil
}

// Stop stops the BMP component
func (c *Component) Stop() error {
	defer func() {
		close(c.peerRemovalChan)
		c.r.Info().Msg("BMP component stopped")
	}()
	c.r.Info().Msg("stopping BMP component")
	c.t.Kill(nil)
	return c.t.Wait()
}
