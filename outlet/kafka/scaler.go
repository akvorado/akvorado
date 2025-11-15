// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"sync"
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

// scalerState is the current FSM state of the scaler.
type scalerState int

const (
	initialState scalerState = iota
	initialIncreaseState
	steadyState
)

// nextWorkerCount calculates the next worker count using dichotomy
func (s *scalerState) nextWorkerCount(request ScaleRequest, currentWorkers, minWorkers, maxWorkers int) int {
	switch *s {
	case initialState:
		switch request {
		case ScaleIncrease:
			*s = initialIncreaseState
			return min(maxWorkers, (currentWorkers+maxWorkers+1)/2)
		case ScaleDecrease:
			return currentWorkers
		}
	case initialIncreaseState:
		switch request {
		case ScaleIncrease:
			return min(maxWorkers, (currentWorkers+maxWorkers+1)/2)
		case ScaleDecrease:
			*s = steadyState
			return max(minWorkers, currentWorkers-1)
		}
	case steadyState:
		switch request {
		case ScaleIncrease:
			return min(maxWorkers, currentWorkers+1)
		case ScaleDecrease:
			return max(minWorkers, currentWorkers-1)
		}
	}
	return currentWorkers
}

// scaleWhileDraining runs a scaling function while draining incoming signals
// from the channel. It spawns two goroutines: one to discard signals and one to
// run the scaling function.
func scaleWhileDraining(ch <-chan ScaleRequest, scaleFn func()) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Go(func() {
		for {
			select {
			case <-done:
				return
			case <-ch:
				// Discard requests
			}
		}
	})
	wg.Go(func() {
		scaleFn()
		close(done)
	})
	wg.Wait()
}

// requestRecord tracks a scale request with its timestamp.
type requestRecord struct {
	request ScaleRequest
	time    time.Time
}

// runScaler starts the automatic scaling loop.
func runScaler(ctx context.Context, config scalerConfiguration) chan<- ScaleRequest {
	ch := make(chan ScaleRequest, config.maxWorkers)
	go func() {
		state := new(scalerState)
		var last time.Time
		var requestHistory []requestRecord
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
				// Between increaseRateLimit and decreaseRateLimit, we accept
				// increase requests.
				if request == ScaleIncrease {
					current := config.getWorkerCount()
					target := state.nextWorkerCount(ScaleIncrease, current, config.minWorkers, config.maxWorkers)
					if target > current {
						scaleWhileDraining(ch, func() {
							config.increaseWorkers(current, target)
						})
					}
					last = time.Now()
					requestHistory = requestHistory[:0]
					continue
				}
				// Between increaseRateLimit and decreaseRateLimit, we also
				// count steady requests to give them a head start.
				if last.Add(config.decreaseRateLimit).After(now) {
					if request == ScaleSteady {
						requestHistory = append(requestHistory, requestRecord{request, now})
					}
					continue
				}
				// Past decreaseRateLimit, we track all requests.
				requestHistory = append(requestHistory, requestRecord{request, now})

				// Remove old requests to prevent unbounded growth. We only
				// consider requests from the last decreaseRateLimit duration to
				// avoid accumulating requests over many hours.
				windowStart := now.Add(-config.decreaseRateLimit)
				i := 0
				for i < len(requestHistory)-1 && requestHistory[i].time.Before(windowStart) {
					i++
				}
				requestHistory = requestHistory[i:]

				// Count decrease vs steady requests in the window.
				var decreaseCount int
				var steadyCount int
				for _, r := range requestHistory {
					switch r.request {
					case ScaleDecrease:
						decreaseCount++
					case ScaleSteady:
						steadyCount++
					}
				}

				// Scale down if we have many decrease requests
				if decreaseCount > steadyCount/2 {
					current := config.getWorkerCount()
					target := state.nextWorkerCount(ScaleDecrease, current, config.minWorkers, config.maxWorkers)
					if target < current {
						scaleWhileDraining(ch, func() {
							config.decreaseWorkers(current, target)
						})
					}
					last = time.Now()
					requestHistory = requestHistory[:0]
				}
			}
		}
	}()
	return ch
}
