// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasource

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

type remoteData struct {
	Name        string `validate:"required"`
	Description string
	Count       int
}

type remoteDataHandler struct {
	data     []remoteData
	fetcher  *Component[remoteData]
	dataLock sync.RWMutex
}

func (h *remoteDataHandler) UpdateData(ctx context.Context, name string, source Source) (int, error) {
	results, err := h.fetcher.Fetch(ctx, name, source)
	if err != nil {
		return 0, err
	}
	h.dataLock.Lock()
	h.data = results
	h.dataLock.Unlock()
	return len(results), nil
}

func TestSource(t *testing.T) {
	// Mux to answer requests
	ready := make(chan bool)
	triggerErrors := atomic.Int32{}
	mux := http.NewServeMux()
	mux.Handle("/data.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case <-ready:
		default:
			w.WriteHeader(404)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		switch triggerErrors.Load() {
		case 0:
			w.WriteHeader(200)
			w.Write([]byte(`
{
  "results": [
    {"name": "foo", "description": "bar"}
  ]
}
`))
		case 1:
			// Validation error
			w.WriteHeader(200)
			w.Write([]byte(`
{
  "results": [
    {"description": "bar"}
  ]
}
`))
		case 2:
			// Map error
			w.WriteHeader(200)
			w.Write([]byte(`
{
  "results": [
    {"name": "foo", "description": "bar", "count": "stuff"}
  ]
}
`))
		case 3:
			// JSON error
			w.WriteHeader(200)
			w.Write([]byte(`
{
  results": [
    {"name": "foo", "description": "bar"}
  ]
}
`))
		case 4:
			// Status error
			w.WriteHeader(500)
			w.Write([]byte(`
{
  results": [
    {"name": "foo", "description": "bar"}
  ]
}
`))
		}
	}))

	// Setup an HTTP server to serve the JSON
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	server := &http.Server{
		Addr:    listener.Addr().String(),
		Handler: mux,
	}
	address := listener.Addr()
	go server.Serve(listener)
	defer server.Shutdown(context.Background())

	r := reporter.NewMock(t)
	config := map[string]Source{
		"local": {
			URL:    fmt.Sprintf("http://%s/data.json", address),
			Method: "GET",
			Headers: map[string]string{
				"X-Foo": "hello",
			},
			Timeout:   20 * time.Millisecond,
			Interval:  20 * time.Millisecond,
			Transform: MustParseTransformQuery(".results[]"),
		},
	}
	handler := remoteDataHandler{
		data: []remoteData{},
	}
	expected := []remoteData{}
	handler.fetcher, _ = New[remoteData](r, handler.UpdateData, "test", config)

	handler.fetcher.Start()
	defer handler.fetcher.Stop()

	// When not ready
	handler.dataLock.RLock()
	if diff := helpers.Diff(handler.data, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
	handler.dataLock.RUnlock()

	close(ready)
	time.Sleep(50 * time.Millisecond)

	// When ready
	expected = []remoteData{
		{
			Name:        "foo",
			Description: "bar",
		},
	}

	handler.dataLock.RLock()
	if diff := helpers.Diff(handler.data, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
	handler.dataLock.RUnlock()

	gotMetrics := r.GetMetrics("akvorado_common_remotedatasource_")
	updates, _ := strconv.Atoi(gotMetrics[`updates_total{source="local",type="test"}`])
	errorsHTTP, _ := strconv.Atoi(
		gotMetrics[`errors_total{error="unexpected HTTP status code",source="local",type="test"}`])
	expectedMetrics := map[string]string{
		`data_total{source="local",type="test"}`:    "1",
		`updates_total{source="local",type="test"}`: strconv.Itoa(max(updates, 1)),
	}
	delete(gotMetrics, `errors_total{error="unexpected HTTP status code",source="local",type="test"}`)
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// Let's add errors
	triggerErrors.Add(1)
	time.Sleep(50 * time.Millisecond)
	triggerErrors.Add(1)
	time.Sleep(50 * time.Millisecond)
	triggerErrors.Add(1)
	time.Sleep(50 * time.Millisecond)
	triggerErrors.Add(1)
	time.Sleep(50 * time.Millisecond)

	gotMetrics = r.GetMetrics("akvorado_common_remotedatasource_")
	updates2, _ := strconv.Atoi(gotMetrics[`updates_total{source="local",type="test"}`])
	errorsHTTP2, _ := strconv.Atoi(
		gotMetrics[`errors_total{error="unexpected HTTP status code",source="local",type="test"}`])
	errorsJSON, _ := strconv.Atoi(
		gotMetrics[`errors_total{error="cannot decode JSON",source="local",type="test"}`])
	errorsMap, _ := strconv.Atoi(
		gotMetrics[`errors_total{error="cannot map JSON",source="local",type="test"}`])
	errorsValidate, _ := strconv.Atoi(
		gotMetrics[`errors_total{error="cannot validate checks",source="local",type="test"}`])
	expectedMetrics = map[string]string{
		`data_total{source="local",type="test"}`:                                       "1",
		`updates_total{source="local",type="test"}`:                                    strconv.Itoa(max(updates2, updates)),
		`errors_total{error="unexpected HTTP status code",source="local",type="test"}`: strconv.Itoa(max(errorsHTTP2, errorsHTTP+1)),
		`errors_total{error="cannot decode JSON",source="local",type="test"}`:          strconv.Itoa(max(errorsJSON, 1)),
		`errors_total{error="cannot map JSON",source="local",type="test"}`:             strconv.Itoa(max(errorsMap, 1)),
		`errors_total{error="cannot validate checks",source="local",type="test"}`:      strconv.Itoa(max(errorsValidate, 1)),
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}
