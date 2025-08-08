// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import "golang.org/x/time/rate"

// ScaleRequest is a request to scale the workers
type ScaleRequest int

const (
	// ScaleIncrease is a request to increase the number of workers
	ScaleIncrease ScaleRequest = iota + 1
	// ScaleDecrease is a request to decrease the number of workers
	ScaleDecrease
)

// startScaler starts the automatic scaling loop
func (c *realComponent) startScaler() chan<- ScaleRequest {
	ch := make(chan ScaleRequest, c.config.MaxWorkers)
	down := rate.Sometimes{Interval: c.config.WorkerDecreaseRateLimit}
	up := rate.Sometimes{Interval: c.config.WorkerIncreaseRateLimit}
	c.t.Go(func() error {
		for {
			select {
			case <-c.t.Dying():
				return nil
			case request := <-ch:
				switch request {
				case ScaleIncrease:
					up.Do(func() {
						if err := c.startOneWorker(); err != nil {
							c.r.Err(err).Msg("cannot spawn a new worker")
						}
					})
				case ScaleDecrease:
					down.Do(func() {
						c.stopOneWorker()
					})
				}
			}
		}
	})
	return ch
}
