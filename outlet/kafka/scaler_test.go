// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"testing"

	"akvorado/common/helpers"
)

func TestScalerDichotomy(t *testing.T) {
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
