// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// ScaleRequest is a request to scale the workers
type ScaleRequest int

const (
	// ScaleIncrease is a request to increase the number of workers
	ScaleIncrease ScaleRequest = iota + 1
	// ScaleDecrease is a request to decrease the number of workers
	ScaleDecrease
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
	down := rate.Sometimes{Interval: config.decreaseRateLimit}
	up := rate.Sometimes{Interval: config.increaseRateLimit}
	go func() {
		state := new(scalerState)
		for {
			select {
			case <-ctx.Done():
				return
			case request := <-ch:
				switch request {
				case ScaleIncrease:
					up.Do(func() {
						current := config.getWorkerCount()
						target := state.nextWorkerCount(request, current, config.minWorkers, config.maxWorkers)
						if target > current {
							config.increaseWorkers(current, target)
						}
					})
				case ScaleDecrease:
					down.Do(func() {
						current := config.getWorkerCount()
						target := state.nextWorkerCount(request, current, config.minWorkers, config.maxWorkers)
						if target < current {
							config.decreaseWorkers(current, target)
						}
					})
				}
			}
		}
	}()
	return ch
}
