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

func TestScalerWithoutRateLimiter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		minWorkers int
		maxWorkers int
		requests   []ScaleRequest
		expected   []int
	}{
		{
			name:       "scale up",
			minWorkers: 1,
			maxWorkers: 16,
			requests:   []ScaleRequest{ScaleIncrease},
			expected:   []int{9},
		}, {
			name:       "scale up twice",
			minWorkers: 1,
			maxWorkers: 16,
			requests:   []ScaleRequest{ScaleIncrease, ScaleIncrease},
			expected:   []int{9, 13},
		}, {
			name:       "scale up many times",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleIncrease, ScaleIncrease, ScaleIncrease, ScaleIncrease,
				ScaleIncrease, ScaleIncrease,
			},
			expected: []int{9, 13, 15, 16},
		}, {
			name:       "scale up twice, then down",
			minWorkers: 1,
			maxWorkers: 16,
			requests:   []ScaleRequest{ScaleIncrease, ScaleIncrease, ScaleDecrease},
			expected:   []int{9, 13, 11},
		},
		// No more tests, the state logic is tested in TestScalerState
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				var mu sync.Mutex
				currentWorkers := tc.minWorkers
				got := []int{}
				config := scalerConfiguration{
					minWorkers:        tc.minWorkers,
					maxWorkers:        tc.maxWorkers,
					increaseRateLimit: time.Second,
					decreaseRateLimit: time.Second,
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
				for _, req := range tc.requests {
					ch <- req
					time.Sleep(5 * time.Second)
				}
				if diff := helpers.Diff(got, tc.expected); diff != "" {
					t.Fatalf("runScaler() (-got, +want):\n%s", diff)
				}
			})
		})
	}
}

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
			increaseRateLimit: time.Second,
			decreaseRateLimit: time.Second,
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
		// Collapsing increases
		for range 10 {
			ch <- ScaleIncrease
			time.Sleep(10 * time.Millisecond)
		}
		if diff := helpers.Diff(got, []int{8}); diff != "" {
			t.Fatalf("runScaler() (-got, +want):\n%s", diff)
		}
		// Collapsing decreases
		for range 10 {
			ch <- ScaleDecrease
			time.Sleep(10 * time.Millisecond)
		}
		if diff := helpers.Diff(got, []int{8, 4}); diff != "" {
			t.Fatalf("runScaler() (-got, +want):\n%s", diff)
		}
		// Still no increase
		ch <- ScaleIncrease
		time.Sleep(10 * time.Millisecond)
		if diff := helpers.Diff(got, []int{8, 4}); diff != "" {
			t.Fatalf("runScaler() (-got, +want):\n%s", diff)
		}
		// Rearm increase rate limiter
		time.Sleep(900 * time.Millisecond)
		ch <- ScaleIncrease
		time.Sleep(10 * time.Millisecond)
		if diff := helpers.Diff(got, []int{8, 4, 6}); diff != "" {
			t.Fatalf("runScaler() (-got, +want):\n%s", diff)
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
			expected: []int{9, 13, 11, 10, 9, 5, 3, 2},
		},
		{
			name:       "down, up, up, down, down, down, down, down, down",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleDecrease,
				ScaleIncrease, ScaleIncrease,
				ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease,
			},
			expected: []int{1, 9, 13, 11, 10, 9, 5, 3, 2},
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
		{
			name:       "reach min",
			minWorkers: 1,
			maxWorkers: 16,
			requests: []ScaleRequest{
				ScaleIncrease,
				ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease, ScaleDecrease,
			},
			expected: []int{9, 5, 3, 2, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := new(scalerState)
			results := []int{}

			for _, req := range tt.requests {
				next := state.nextWorkerCount(req, tt.minWorkers, tt.maxWorkers)
				results = append(results, next)
			}

			if diff := helpers.Diff(results, tt.expected); diff != "" {
				t.Fatalf("nextWorkerCount() (-got, +want):\n%s", diff)
			}
		})
	}
}
