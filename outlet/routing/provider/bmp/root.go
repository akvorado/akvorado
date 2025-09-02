// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package bmp provides a BMP server to receive BGP routes from
// various exporters.
package bmp

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/benbjohnson/clock"
	"gopkg.in/tomb.v2"

	"akvorado/common/reporter"
	"akvorado/outlet/routing/provider"
)

// Provider represents the BMP provider.
type Provider struct {
	r           *reporter.Reporter
	d           *Dependencies
	t           tomb.Tomb
	config      Configuration
	acceptedRDs map[RD]struct{}
	active      atomic.Bool

	address net.Addr
	metrics metrics

	// RIB management with peers
	rib               *rib
	peers             map[peerKey]*peerInfo
	lastPeerReference uint32
	staleTimer        *clock.Timer
	mu                sync.RWMutex
}

// Dependencies define the dependencies of the BMP component.
type Dependencies = provider.Dependencies

var (
	_ provider.Provider      = &Provider{}
	_ provider.Configuration = Configuration{}
)

// New creates a new BMP component from its configuration.
func (configuration Configuration) New(r *reporter.Reporter, dependencies Dependencies) (provider.Provider, error) {
	if dependencies.Clock == nil {
		dependencies.Clock = clock.New()
	}
	p := Provider{
		r:      r,
		d:      &dependencies,
		config: configuration,

		rib:   newRIB(),
		peers: make(map[peerKey]*peerInfo),
	}
	if len(p.config.RDs) > 0 {
		p.acceptedRDs = make(map[RD]struct{})
		for _, rd := range p.config.RDs {
			p.acceptedRDs[rd] = struct{}{}
		}
	}
	p.staleTimer = p.d.Clock.AfterFunc(time.Hour, p.removeStalePeers)

	p.d.Daemon.Track(&p.t, "outlet/bmp")
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
			tcpConn := conn.(*net.TCPConn)
			remote := conn.RemoteAddr().(*net.TCPAddr)
			exporterIP, _ := netip.AddrFromSlice(remote.IP)
			exporter := netip.AddrPortFrom(exporterIP, uint16(remote.Port))
			exporterStr := exporter.Addr().Unmap().String()
			if p.config.ReceiveBuffer > 0 {
				if err := tcpConn.SetReadBuffer(int(p.config.ReceiveBuffer)); err != nil {
					p.r.Warn().
						Str("error", err.Error()).
						Str("listen", p.config.Listen).
						Msgf("unable to set requested TCP receive buffer size (%d bytes)", p.config.ReceiveBuffer)
				}
			}
			// Verify the buffer size was actually set correctly
			if syscallConn, err := tcpConn.SyscallConn(); err == nil {
				var actualSize int
				syscallConn.Control(func(fd uintptr) {
					if val, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF); err == nil {
						actualSize = val
					}
				})
				p.metrics.bufferSize.WithLabelValues(exporterStr).Set(float64(actualSize))
				if p.config.ReceiveBuffer > 0 && actualSize < int(p.config.ReceiveBuffer) {
					p.r.Warn().
						Str("listen", p.config.Listen).
						Int("requested", int(p.config.ReceiveBuffer)).
						Int("actual", actualSize).
						Msg("TCP receive buffer size was capped by system limits (check net.core.rmem_max)")
				}
			}
			p.active.Store(true)
			p.t.Go(func() error {
				return p.serveConnection(tcpConn, exporter, exporterStr)
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
	defer p.r.Info().Msg("BMP component stopped")
	p.r.Info().Msg("stopping BMP component")
	p.t.Kill(nil)
	return p.t.Wait()
}
