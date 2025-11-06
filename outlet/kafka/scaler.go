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

// scalerState tracks the state of the dichotomy search
type scalerState struct {
	searchMin int  // current search minimum bound
	searchMax int  // current search maximum bound
	current   int  // current number of workers
	previous  int  // previous number of workers before last change
	goingUp   bool // current direction: nil=uninitialized, true=up, false=down
}

// nextWorkerCount calculates the next worker count using dichotomy
func (s *scalerState) nextWorkerCount(request ScaleRequest, minWorkers, maxWorkers int) int {
	if s.searchMin == 0 {
		*s = scalerState{
			searchMin: minWorkers,
			searchMax: maxWorkers,
			current:   minWorkers,
			previous:  minWorkers,
			goingUp:   true,
		}
	}
	requestUp := (request == ScaleIncrease)

	// On direction change, narrow search space
	if s.goingUp != requestUp {
		s.goingUp = requestUp

		if requestUp {
			// Changed to going up: search between [current, searchMax]
			s.searchMin = s.current
		} else {
			// Changed to going down: search between [previous, current]
			s.searchMin = s.previous
			s.searchMax = s.current
		}
	}

	// Calculate next value as midpoint of search space
	var next int
	if requestUp {
		next = (s.current + s.searchMax + 1) / 2 // ceiling
	} else {
		next = (s.searchMin + s.current) / 2 // floor
	}

	// If we can't move (hit the limit), expand search to full bounds
	if next == s.current {
		if requestUp && s.searchMax < maxWorkers {
			s.searchMax = maxWorkers
			next = (s.current + s.searchMax + 1) / 2
		} else if !requestUp && s.searchMin > minWorkers {
			s.searchMin = minWorkers
			next = (s.searchMin + s.current) / 2
		}
	}

	// Update state for next iteration
	s.previous = s.current
	s.current = next
	return next
}

// runScaler starts the automatic scaling loop
func (c *realComponent) runScaler() chan<- ScaleRequest {
	ch := make(chan ScaleRequest, c.config.MaxWorkers)
	down := rate.Sometimes{Interval: c.config.WorkerDecreaseRateLimit}
	up := rate.Sometimes{Interval: c.config.WorkerIncreaseRateLimit}
	c.t.Go(func() error {
		state := new(scalerState)
		for {
			select {
			case <-c.t.Dying():
				return nil
			case request := <-ch:
				switch request {
				case ScaleIncrease:
					up.Do(func() {
						c.workerMu.Lock()
						currentWorkers := len(c.workers)
						c.workerMu.Unlock()

						targetWorkers := state.nextWorkerCount(request, c.config.MinWorkers, c.config.MaxWorkers)

						if targetWorkers > currentWorkers {
							c.r.Info().Msgf("increase number of workers from %d to %d", currentWorkers, targetWorkers)
							for i := currentWorkers; i < targetWorkers; i++ {
								if err := c.startOneWorker(); err != nil {
									c.r.Err(err).Msg("cannot spawn a new worker")
									return
								}
							}
						}
					})
				case ScaleDecrease:
					down.Do(func() {
						c.workerMu.Lock()
						currentWorkers := len(c.workers)
						c.workerMu.Unlock()

						targetWorkers := state.nextWorkerCount(request, c.config.MinWorkers, c.config.MaxWorkers)

						if targetWorkers < currentWorkers {
							c.r.Info().Msgf("decrease number of workers from %d to %d", currentWorkers, targetWorkers)
							for i := currentWorkers; i > targetWorkers; i-- {
								c.stopOneWorker()
							}
						}
					})
				}
			}
		}
	})
	return ch
}
