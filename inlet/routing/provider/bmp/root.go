// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package bmp provides a BMP server to receive BGP routes from
// various exporters.
package bmp

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
	"gopkg.in/tomb.v2"

	"akvorado/common/helpers/sync"
	"akvorado/common/reporter"
	"akvorado/inlet/routing/provider"
)

// Provider represents the BMP provider.
type Provider struct {
	r           *reporter.Reporter
	d           *Dependencies
	t           tomb.Tomb
	config      Configuration
	acceptedRDs map[uint64]struct{}
	active      atomic.Bool

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
type Dependencies = provider.Dependencies

// New creates a new BMP component from its configuration.
func (configuration Configuration) New(r *reporter.Reporter, dependencies Dependencies) (provider.Provider, error) {
	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	p := Provider{
		r:      r,
		d:      &dependencies,
		config: configuration,

		rib:             newRIB(),
		peers:           make(map[peerKey]*peerInfo),
		peerRemovalChan: make(chan peerKey, configuration.RIBPeerRemovalMaxQueue),
	}
	if len(p.config.RDs) > 0 {
		p.acceptedRDs = make(map[uint64]struct{})
		for _, rd := range p.config.RDs {
			p.acceptedRDs[uint64(rd)] = struct{}{}
		}
	}
	p.staleTimer = p.d.Clock.AfterFunc(time.Hour, p.removeStalePeers)

	p.d.Daemon.Track(&p.t, "inlet/bmp")
	p.initMetrics()
	return &p, nil
}

// Start starts the BMP provider.
func (p *Provider) Start() error {
	p.r.Info().Msg("starting BMP provider")
	listener, err := net.Listen("tcp", p.config.Listen)
	if err != nil {
		return fmt.Errorf("unable to listen to %v: %w", p.config.Listen, err)
	}
	p.address = listener.Addr()

	// Peer removal
	p.t.Go(p.peerRemovalWorker)

	// Listener
	p.t.Go(func() error {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if p.t.Alive() {
					return fmt.Errorf("cannot accept new connection: %w", err)
				}
				return nil
			}
			p.active.Store(true)
			p.t.Go(func() error {
				return p.serveConnection(conn.(*net.TCPConn))
			})
		}
	})
	p.t.Go(func() error {
		<-p.t.Dying()
		listener.Close()
		return nil
	})
	return nil
}

// Stop stops the BMP provider.
func (p *Provider) Stop() error {
	defer func() {
		close(p.peerRemovalChan)
		p.r.Info().Msg("BMP component stopped")
	}()
	p.r.Info().Msg("stopping BMP component")
	p.t.Kill(nil)
	return p.t.Wait()
}
