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

// scalerState tracks the state of the dichotomy search
type scalerState struct {
	searchMin int  // current search minimum bound
	searchMax int  // current search maximum bound
	previous  int  // previous number of workers before last change
	goingUp   bool // current direction: nil=uninitialized, true=up, false=down
}

// nextWorkerCount calculates the next worker count using dichotomy
func (s *scalerState) nextWorkerCount(request ScaleRequest, currentWorkers, minWorkers, maxWorkers int) int {
	if s.searchMin == 0 {
		*s = scalerState{
			searchMin: minWorkers,
			searchMax: maxWorkers,
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
			s.searchMin = currentWorkers
		} else {
			// Changed to going down: search between [previous, current]
			s.searchMin = s.previous
			s.searchMax = currentWorkers
		}
	}

	// Calculate next value as midpoint of search space
	var next int
	if requestUp {
		next = (currentWorkers + s.searchMax + 1) / 2 // ceiling
	} else {
		next = (s.searchMin + currentWorkers) / 2 // floor
	}

	// If we can't move (hit the limit), expand search to full bounds
	if next == currentWorkers {
		if requestUp && s.searchMax < maxWorkers {
			s.searchMax = maxWorkers
			next = (currentWorkers + s.searchMax + 1) / 2
		} else if !requestUp && s.searchMin > minWorkers {
			s.searchMin = minWorkers
			next = (s.searchMin + currentWorkers) / 2
		}
	}

	// Update state for next iteration
	s.previous = currentWorkers
	return next
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
