// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package reporter

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// NewMock creates a new reporter for tests. Currently, this is the same as a
// production reporter, except when running benchmarks.
func NewMock(t testing.TB) *Reporter {
	t.Helper()
	config := DefaultConfiguration()
	r, err := New(config)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if _, ok := t.(*testing.B); ok {
		r.Logger.Logger = r.Logger.Level(zerolog.WarnLevel)
	}
	return r
}

// GetMetrics returns a map from metric name to its value (as a
// string). It keeps only metrics matching the provided prefix.
func (r *Reporter) GetMetrics(prefix string, subset ...string) map[string]string {
	results := make(map[string]string)
	req := httptest.NewRequest("GET", "/api/v0/metrics", nil)
	w := httptest.NewRecorder()
	r.MetricsHTTPHandler().ServeHTTP(w, req)

	lines := strings.Split(w.Body.String(), "\n")
outer:
	for _, line := range lines {
		// Very basic parsing
		if strings.HasPrefix(line, "#") || !strings.HasPrefix(line, prefix) {
			continue
		}
		var result []string
		if idx := strings.Index(line, "} "); idx >= 0 {
			result = []string{line[:idx+1], line[idx+2:]}
		} else {
			result = strings.SplitN(line, " ", 2)
			if len(result) != 2 {
				continue
			}
		}
		trimmed := strings.TrimPrefix(result[0], prefix)
		nonnegative := 0
		if len(subset) > 0 {
			for _, oPrefix := range subset {
				if oPrefix[0] == '-' {
					if strings.HasPrefix(trimmed, oPrefix[1:]) {
						continue outer
					}
				} else {
					nonnegative++
					if strings.HasPrefix(trimmed, oPrefix) {
						results[trimmed] = result[1]
						break
					}
				}
			}
		}
		if nonnegative == 0 {
			results[trimmed] = result[1]
		}
	}

	return results
}
