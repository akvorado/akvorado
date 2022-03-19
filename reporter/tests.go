//go:build !release

package reporter

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// NewMock creates a new reporter for tests. Currently, this is the same as a production reporter.
func NewMock(t *testing.T) *Reporter {
	t.Helper()
	r, err := New(Configuration{})
	if err != nil {
		t.Errorf("New() error:\n%+v", err)
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
		if len(subset) > 0 {
			for _, oPrefix := range subset {
				if strings.HasPrefix(trimmed, oPrefix) {
					results[trimmed] = result[1]
					break
				}
			}
		} else {
			results[trimmed] = result[1]
		}
	}

	return results
}
