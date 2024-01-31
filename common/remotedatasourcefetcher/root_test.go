// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasourcefetcher

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

type remoteData struct {
	name        string
	description string
}

type remoteDataHandler struct {
	data     []remoteData
	fetcher  *Component[remoteData]
	dataLock sync.RWMutex
}

func (h *remoteDataHandler) UpdateData(ctx context.Context, name string, source RemoteDataSource) (int, error) {
	results, err := h.fetcher.Fetch(ctx, name, source)
	if err != nil {
		return 0, err
	}
	h.dataLock.Lock()
	h.data = results
	h.dataLock.Unlock()
	return len(results), nil
}

func TestRemoteDataSourceFetcher(t *testing.T) {
	// Mux to answer requests
	ready := make(chan bool)
	mux := http.NewServeMux()
	mux.Handle("/data.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ready:
		default:
			w.WriteHeader(404)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`
{
  "results": [
    {"name": "foo", "description": "bar"}
  ]
}
`))
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
	config := map[string]RemoteDataSource{
		"local": {
			URL:    fmt.Sprintf("http://%s/data.json", address),
			Method: "GET",
			Headers: map[string]string{
				"X-Foo": "hello",
			},
			Timeout:  20 * time.Millisecond,
			Interval: 100 * time.Millisecond,
			Transform: MustParseTransformQuery(`
.results[]
`),
		},
	}
	handler := remoteDataHandler{
		data: []remoteData{},
	}
	var expected []remoteData
	handler.fetcher, _ = New[remoteData](r, handler.UpdateData, "test", config)

	handler.fetcher.Start()

	handler.dataLock.RLock()
	if diff := helpers.Diff(handler.data, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
	handler.dataLock.RUnlock()

	// before ready

	close(ready)
	time.Sleep(50 * time.Millisecond)

	expected = []remoteData{
		{
			name:        "foo",
			description: "bar",
		},
	}

	handler.dataLock.RLock()
	if diff := helpers.Diff(handler.data, expected); diff != "" {
		t.Fatalf("static provider (-got, +want):\n%s", diff)
	}
	handler.dataLock.RUnlock()

	gotMetrics := r.GetMetrics("akvorado_common_remotedatasourcefetcher_data_")
	expectedMetrics := map[string]string{
		`total{source="local",type="test"}`: "1",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}

	// We now should be able to resolve our remote data from remote source

}
