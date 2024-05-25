// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"time"
)

func (p *Provider) peerRemovalWorker() error {
	for {
		select {
		case <-p.t.Dying():
			return nil
		case pkey := <-p.peerRemovalChan:
			exporterStr := pkey.exporter.Addr().Unmap().String()
			for {
				// Do one run of removal (read/write lock)
				removed, done, duplicate := func() (int, bool, bool) {
					start := p.d.Clock.Now()
					ctx, cancel := context.WithTimeout(p.t.Context(context.Background()),
						p.config.RIBPeerRemovalMaxTime)
					p.mu.Lock()
					defer func() {
						cancel()
						p.mu.Unlock()
						p.metrics.locked.WithLabelValues("peer-removal").Observe(
							float64(p.d.Clock.Now().Sub(start).Nanoseconds()) / 1000 / 1000 / 1000)
					}()
					pinfo := p.peers[pkey]
					if pinfo == nil {
						// Already removed (removal can be queued several times)
						return 0, true, true
					}
					removed, done := p.rib.flushPeerContext(ctx, pinfo.reference,
						p.config.RIBPeerRemovalBatchRoutes)
					if done {
						// Run was complete, remove the peer (we need the lock)
						delete(p.peers, pkey)
					}
					return removed, done, false
				}()

				// Update stats and optionally sleep
				p.metrics.routes.WithLabelValues(exporterStr).Sub(float64(removed))
				if done {
					// Run was complete, update metrics
					if !duplicate {
						p.metrics.peers.WithLabelValues(exporterStr).Dec()
						p.metrics.peerRemovalDone.WithLabelValues(exporterStr).Inc()
					}
					break
				}
				// Run is incomplete, update metrics and sleep a bit
				p.metrics.peerRemovalPartial.WithLabelValues(exporterStr).Inc()
				select {
				case <-p.t.Dying():
					return nil
				case <-time.After(p.config.RIBPeerRemovalSleepInterval):
				}
			}
		}
	}
}
