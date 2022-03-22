package reporter

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthcheckStatus represents an healthcheck status.
type HealthcheckStatus int

// HealthcheckResult combines a status and a reason
type HealthcheckResult struct {
	Status HealthcheckStatus `json:"status"`
	Reason string            `json:"reason"`
}

const (
	// HealthcheckOK says "OK"
	HealthcheckOK HealthcheckStatus = iota
	// HealthcheckWarning says there is a non-fatal condition
	HealthcheckWarning
	// HealthcheckError says there is a big problem with the component
	HealthcheckError
)

func (hs HealthcheckStatus) String() string {
	switch hs {
	case HealthcheckOK:
		return "ok"
	case HealthcheckWarning:
		return "warning"
	case HealthcheckError:
		return "error"
	default:
		return "unknown"
	}
}

// MarshalText turns a status into text.
func (hs HealthcheckStatus) MarshalText() ([]byte, error) {
	return []byte(hs.String()), nil
}

// HealthcheckFunc defines a function returning an healthcheck result.
type HealthcheckFunc func(context.Context) HealthcheckResult

// RegisterHealthcheck registers a new healthcheck. An healthcheck is
// a function returning a state and a status string.
func (r *Reporter) RegisterHealthcheck(name string, hf HealthcheckFunc) {
	r.healthchecksLock.Lock()
	r.healthchecks[name] = hf
	r.healthchecksLock.Unlock()
}

// RunHealthchecks execute all healthchecks in parallel and returns a
// global status as well as a map from service names to returned
// results.
func (r *Reporter) RunHealthchecks(ctx context.Context) (HealthcheckStatus, map[string]HealthcheckResult) {
	var wg sync.WaitGroup
	results := make(map[string]HealthcheckResult)

	r.healthchecksLock.Lock()
	defer r.healthchecksLock.Unlock()
	runningHealthchecks := len(r.healthchecks)
	if runningHealthchecks == 0 {
		return HealthcheckOK, results
	}

	// Go routine to centralize results
	type oneResult struct {
		name   string
		result HealthcheckResult
	}
	resultChan := make(chan oneResult)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-resultChan:
				results[result.name] = result.result
				runningHealthchecks--
				if runningHealthchecks == 0 {
					return
				}
			}
		}
	}()

	// One goroutine for each healthcheck
	for name, healthcheckFunc := range r.healthchecks {
		wg.Add(1)
		go func(name string, healthcheckFunc HealthcheckFunc) {
			defer wg.Done()
			result := healthcheckFunc(ctx)
			oneResult := oneResult{
				name:   name,
				result: result,
			}
			select {
			case <-ctx.Done():
			case resultChan <- oneResult:
			}
		}(name, healthcheckFunc)
	}

	wg.Wait() // keep lock, we don't want something to change

	// Check what we have
	currentStatus := HealthcheckOK
	for name := range r.healthchecks {
		if result, ok := results[name]; ok {
			if result.Status > currentStatus {
				currentStatus = result.Status
			}
		} else {
			results[name] = HealthcheckResult{HealthcheckError, "timeout during check"}
			currentStatus = HealthcheckError
		}
	}

	return currentStatus, results
}

// HealthcheckHTTPHandler is an HTTP handler return healthcheck results as JSON.
func (r *Reporter) HealthcheckHTTPHandler() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			globalStatus, details := r.RunHealthchecks(ctx)
			results := map[string]interface{}{
				"status":  globalStatus,
				"details": details,
			}
			w.Header().Set("Content-Type", "application/json")
			if globalStatus == HealthcheckError {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			json.NewEncoder(w).Encode(results)
		})
}
