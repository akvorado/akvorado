// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"context"
	"time"
)

func (c *Component) peerRemovalWorker() error {
	for {
		select {
		case <-c.t.Dying():
			return nil
		case pkey := <-c.peerRemovalChan:
			exporterStr := pkey.exporter.Addr().Unmap().String()
			for {
				// Do one run of removal.
				removed, done := func() (int, bool) {
					ctx, cancel := context.WithTimeout(c.t.Context(context.Background()),
						c.config.PeerRemovalMaxTime)
					defer cancel()
					start := c.d.Clock.Now()
					c.mu.Lock()
					defer func() {
						c.mu.Unlock()
						c.metrics.locked.WithLabelValues("peer-removal").Observe(
							float64(c.d.Clock.Now().Sub(start).Nanoseconds()) / 1000 / 1000 / 1000)
					}()
					pinfo := c.peers[pkey]
					if pinfo == nil {
						// Already removed (removal can be queued several times)
						return 0, true
					}
					removed, done := c.rib.flushPeer(ctx, pinfo.reference, c.config.PeerRemovalMinRoutes)
					if done {
						// Run was complete, remove the peer (we need the lock)
						delete(c.peers, pkey)
					}
					return removed, done
				}()
				c.metrics.routes.WithLabelValues(exporterStr).Sub(float64(removed))
				if done {
					// Run was complete, update metrics
					c.metrics.peers.WithLabelValues(exporterStr).Dec()
					c.metrics.peerRemovalDone.WithLabelValues(exporterStr).Inc()
					break
				}
				// Run is incompletem, update metrics and sleep a bit
				c.metrics.peerRemovalPartial.WithLabelValues(exporterStr).Inc()
				select {
				case <-c.t.Dying():
					return nil
				case <-time.After(c.config.PeerRemovalSleepInterval):
				}
			}
		}
	}
}
