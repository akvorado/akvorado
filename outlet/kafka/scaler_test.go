// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"akvorado/common/helpers"
)

func TestScalerRateLimiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var mu sync.Mutex
		currentWorkers := 1
		got := []int{}
		config := scalerConfiguration{
			minWorkers:        1,
			maxWorkers:        15,
			increaseRateLimit: time.Minute,
			decreaseRateLimit: 5 * time.Minute,
			getWorkerCount: func() int {
				mu.Lock()
				defer mu.Unlock()
				return currentWorkers
			},
			increaseWorkers: func(from, to int) {
				t.Logf("increaseWorkers(from: %d, to: %d)", from, to)
				mu.Lock()
				defer mu.Unlock()
				got = append(got, to)
				currentWorkers = to
			},
			decreaseWorkers: func(from, to int) {
				t.Logf("decreaseWorkers(from: %d, to: %d)", from, to)
				mu.Lock()
				defer mu.Unlock()
				got = append(got, to)
				currentWorkers = to
			},
		}
		ch := runScaler(ctx, config)
		check := func(expected []int) {
			t.Helper()
			time.Sleep(time.Millisecond)
			mu.Lock()
			defer mu.Unlock()
			if diff := helpers.Diff(got, expected); diff != "" {
				t.Fatalf("runScaler() (-got, +want):\n%s", diff)
			}
		}
		// Increase on first scale request
		ch <- ScaleIncrease
		check([]int{8})

		// Collapsing further increases
		for range 10 {
			time.Sleep(5 * time.Second)
			ch <- ScaleIncrease
		}
		// time == 50 seconds
		check([]int{8})

		// Then increase again
		time.Sleep(10 * time.Second)
		ch <- ScaleIncrease
		// time = 1 minute
		check([]int{8, 12})

		// Do not decrease (too soon)
		for range 10 {
			time.Sleep(6 * time.Second)
			ch <- ScaleDecrease
		}
		// time = 1 minute
		check([]int{8, 12})

		// Do not decrease even after 4 minutes
		for range 39 {
			time.Sleep(6 * time.Second)
			ch <- ScaleDecrease
		}
		// time = 4m54
		check([]int{8, 12})

		// Decrease (5-second timeout done)
		time.Sleep(6 * time.Second)
		ch <- ScaleDecrease
		// time = 5 minutes
		check([]int{8, 12, 11})

		// Do not increase
		for range 10 {
			time.Sleep(5 * time.Second)
			ch <- ScaleIncrease
		}
		// time = 50 seconds
		check([]int{8, 12, 11})

		// Increase after 10 more seconds
		time.Sleep(10 * time.Second)
		ch <- ScaleIncrease
		// time = 1 minute
		check([]int{8, 12, 11, 12})

		// When mixing increase and decrease, increase
		for range 60 {
			time.Sleep(time.Second)
			ch <- ScaleIncrease
			ch <- ScaleDecrease
		}
		// time = 1 minute
		check([]int{8, 12, 11, 12, 13})

		// When we only have a few increase at the beginning, but mostly decrease after that, decrease
		time.Sleep(55 * time.Second)
		ch <- ScaleIncrease
		ch <- ScaleIncrease
		ch <- ScaleIncrease
		ch <- ScaleIncrease
		for range 295 {
			time.Sleep(time.Second)
			ch <- ScaleDecrease
		}
		check([]int{8, 12, 11, 12, 13, 12})

		// If we have one decrease request after 5 minutes, decrease
		time.Sleep(5 * time.Minute)
		for range 10 {
			ch <- ScaleDecrease
		}
		check([]int{8, 12, 11, 12, 13, 12, 11})

		// But more likely, we have steady requests, then decrease requests
		time.Sleep(time.Minute)
		for range 240 {
			time.Sleep(time.Second)
			ch <- ScaleSteady
		}
		for range 60 {
			time.Sleep(time.Second)
			ch <- ScaleDecrease
		}
		// time=6m, no change (240 vs 60)
		check([]int{8, 12, 11, 12, 13, 12, 11})
		for range 60 {
			time.Sleep(time.Second)
			ch <- ScaleDecrease
		}
		// time=7m, decrease (180 vs 120)
		check([]int{8, 12, 11, 12, 13, 12, 11, 10})
		for range 30 {
			time.Sleep(time.Second)
			ch <- ScaleDecrease
		}

		// We should not account for steady requests for too long!
		time.Sleep(time.Minute)
		for range 2400 {
			time.Sleep(time.Second)
			ch <- ScaleSteady
		}
		// 2400 vs 0
		check([]int{8, 12, 11, 12, 13, 12, 11, 10})
		time.Sleep(time.Second)
		for range 300 {
			ch <- ScaleDecrease
		}
		// 0 vs 300
		check([]int{8, 12, 11, 12, 13, 12, 11, 10, 9})
	})
}

func TestScalerDoesNotBlock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var mu sync.Mutex
		currentWorkers := 1
		scalingInProgress := false

		config := scalerConfiguration{
			minWorkers:        1,
			maxWorkers:        16,
			increaseRateLimit: time.Second,
			decreaseRateLimit: time.Second,
			getWorkerCount: func() int {
				mu.Lock()
				defer mu.Unlock()
				return currentWorkers
			},
			increaseWorkers: func(from, to int) {
				t.Logf("increaseWorkers(from: %d, to: %d) - start", from, to)
				mu.Lock()
				scalingInProgress = true
				mu.Unlock()

				// Simulate a slow scaling operation
				time.Sleep(30 * time.Second)

				mu.Lock()
				currentWorkers = to
				scalingInProgress = false
				mu.Unlock()
				t.Logf("increaseWorkers(from: %d, to: %d) - done", from, to)
			},
			decreaseWorkers: func(from, to int) {
				t.Logf("decreaseWorkers(from: %d, to: %d) - start", from, to)
				mu.Lock()
				scalingInProgress = true
				mu.Unlock()

				// Simulate a slow scaling operation
				time.Sleep(30 * time.Second)

				mu.Lock()
				currentWorkers = to
				scalingInProgress = false
				mu.Unlock()
				t.Logf("decreaseWorkers(from: %d, to: %d) - done", from, to)
			},
		}

		ch := runScaler(ctx, config)

		// Send the first scale request that will trigger a slow scaling operation
		ch <- ScaleIncrease
		time.Sleep(time.Second)

		// Verify scaling is in progress
		mu.Lock()
		if !scalingInProgress {
			t.Fatal("runScaler(): scaling should be in progress")
		}
		mu.Unlock()

		// Now send many more signals while scaling is in progress.
		// These should not block - they should be discarded.
		sendDone := make(chan struct{})
		go func() {
			for range 100 {
				ch <- ScaleIncrease
			}
			close(sendDone)
		}()

		// Wait for all sends to complete with a timeout
		select {
		case <-sendDone:
			t.Log("runScaler(): all signals sent successfully without blocking")
		case <-time.After(5 * time.Second):
			t.Fatal("runScaler(): blocked")
		}

		// Wait for the scaling operation to complete
		time.Sleep(30 * time.Second)

		// Verify scaling completed
		mu.Lock()
		defer mu.Unlock()
		if scalingInProgress {
			t.Fatal("runScaler(): still scaling")
		}
		if currentWorkers != 9 {
			t.Fatalf("runScaler(): expected 9 workers, got %d", currentWorkers)
		}
	})
}

func TestScalerState(t *testing.T) {
	tests := []struct {
		name       string
		minWorkers int
		maxWorkers int
		requests   []ScaleRequest
		expected   []int
	}{
		{
			name:       "simple up",
			minWorkers: 1,
			maxWorkers: 16,
			requests:   []ScaleRequest{ScaleIncrease},
			expected:   []int{9},
		},
		{
			name:       "up, up, up, down, down, up",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleIncrease, ScaleIncrease, ScaleIncrease,
				ScaleDecrease, ScaleDecrease,
				ScaleIncrease},
			expected: []int{9, 13, 15, 14, 13, 14},
		},
		{
			name:       "up, up, down, down, down, down, down, down",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleIncrease, ScaleIncrease,
				ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease,
			},
			expected: []int{9, 13, 12, 11, 10, 9, 8, 7},
		},
		{
			// Ignore first down
			name:       "down, up, up, down, down, down, down, down, down",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleDecrease,
				ScaleIncrease, ScaleIncrease,
				ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease,
			},
			expected: []int{1, 9, 13, 12, 11, 10, 9, 8, 7},
		},
		{
			name:       "simple down from min",
			minWorkers: 1,
			maxWorkers: 16,
			requests:   []ScaleRequest{ScaleDecrease},
			expected:   []int{1},
		},
		{
			name:       "reach max",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleIncrease, ScaleIncrease, ScaleIncrease, ScaleIncrease, ScaleIncrease, ScaleIncrease,
			},
			expected: []int{9, 13, 15, 16, 16, 16},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := new(scalerState)
			current := tt.minWorkers
			results := []int{}

			for _, req := range tt.requests {
				current = state.nextWorkerCount(req, current, tt.minWorkers, tt.maxWorkers)
				results = append(results, current)
			}

			if diff := helpers.Diff(results, tt.expected); diff != "" {
				t.Fatalf("nextWorkerCount() (-got, +want):\n%s", diff)
			}
		})
	}
}
