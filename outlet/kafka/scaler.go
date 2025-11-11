// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"time"
)

// ScaleRequest is a request to scale the workers
type ScaleRequest int

const (
	// ScaleIncrease is a request to increase the number of workers
	ScaleIncrease ScaleRequest = iota + 1
	// ScaleDecrease is a request to decrease the number of workers
	ScaleDecrease
	// ScaleSteady is a request to keep the number of workers as is
	ScaleSteady
)

// scalerConfiguration is the configuration for the scaler subcomponent
type scalerConfiguration struct {
	minWorkers        int
	maxWorkers        int
	increaseRateLimit time.Duration
	decreaseRateLimit time.Duration
	increaseWorkers   func(from, to int)
	decreaseWorkers   func(from, to int)
	getWorkerCount    func() int
}

// scalerState tracks scaler's state. The FSM has two states: starting and
// steady. In starting state, when the scale request is up, we increase the
// number of workers using a dichotomy between the current number and the
// maximum workers. When the scale request is down, we decrease the number of
// workers by one and switch to steady state. In steady state, the number of
// workers is increased by one or decreased by one.
type scalerState struct {
	steady bool // are we in the steady state?
}

// nextWorkerCount calculates the next worker count using dichotomy
func (s *scalerState) nextWorkerCount(request ScaleRequest, currentWorkers, minWorkers, maxWorkers int) int {
	switch s.steady {
	case false:
		// Initial state
		switch request {
		case ScaleIncrease:
			return min(maxWorkers, (currentWorkers+maxWorkers+1)/2)
		case ScaleDecrease:
			s.steady = true
			return max(minWorkers, currentWorkers-1)
		}
	case true:
		// Steady state
		switch request {
		case ScaleIncrease:
			return min(maxWorkers, currentWorkers+1)
		case ScaleDecrease:
			return max(minWorkers, currentWorkers-1)
		}
	}
	return currentWorkers
}

// runScaler starts the automatic scaling loop
func runScaler(ctx context.Context, config scalerConfiguration) chan<- ScaleRequest {
	ch := make(chan ScaleRequest, config.maxWorkers)
	go func() {
		state := new(scalerState)
		var last time.Time
		var decreaseCount int
		for {
			select {
			case <-ctx.Done():
				return
			case request := <-ch:
				now := time.Now()
				// During increaseRateLimit, we ignore everything.
				if last.Add(config.increaseRateLimit).After(now) {
					continue
				}
				// Between increaseRateLimit and decreaseRateLimit, we accept increase requests.
				if request == ScaleIncrease {
					current := config.getWorkerCount()
					target := state.nextWorkerCount(ScaleIncrease, current, config.minWorkers, config.maxWorkers)
					if target > current {
						config.increaseWorkers(current, target)
					}
					last = now
					decreaseCount = 0
					continue
				}
				// We also count steady requests.
				if request == ScaleSteady {
					decreaseCount--
				}
				// But we ignore everything else.
				if last.Add(config.decreaseRateLimit).After(now) {
					continue
				}
				// Past decreaseRateLimit, we count decrease requests and
				// request 10 of them if not cancelled by steady requests (they
				// have a head start).
				if request == ScaleDecrease {
					decreaseCount++
					if decreaseCount >= 10 {
						current := config.getWorkerCount()
						target := state.nextWorkerCount(ScaleDecrease, current, config.minWorkers, config.maxWorkers)
						if target < current {
							config.decreaseWorkers(current, target)
						}
						last = now
						decreaseCount = 0
					}
				}
			}
		}
	}()
	return ch
}
