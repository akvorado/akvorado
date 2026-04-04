// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasource

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
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
	defer server.Shutdown(t.Context())

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
		gotMetrics[`errors_total{error="cannot map result",source="local",type="test"}`])
	errorsValidate, _ := strconv.Atoi(
		gotMetrics[`errors_total{error="cannot validate checks",source="local",type="test"}`])
	expectedMetrics = map[string]string{
		`data_total{source="local",type="test"}`:                                       "1",
		`updates_total{source="local",type="test"}`:                                    strconv.Itoa(max(updates2, updates)),
		`errors_total{error="unexpected HTTP status code",source="local",type="test"}`: strconv.Itoa(max(errorsHTTP2, errorsHTTP+1)),
		`errors_total{error="cannot decode JSON",source="local",type="test"}`:          strconv.Itoa(max(errorsJSON, 1)),
		`errors_total{error="cannot map result",source="local",type="test"}`:           strconv.Itoa(max(errorsMap, 1)),
		`errors_total{error="cannot validate checks",source="local",type="test"}`:      strconv.Itoa(max(errorsValidate, 1)),
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Fatalf("Metrics (-got, +want):\n%s", diff)
	}
}

// generateSelfSignedCert generates a self-signed certificate for testing
func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	// Generate a private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error:\n%+v", err)
	}

	// Create a certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Create the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error:\n%+v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}
}

func TestSourceCSV(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/data.csv", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "text/csv")
		w.WriteHeader(200)
		w.Write([]byte("foo,bar\nbaz,qux\n"))
	}))
	mux.Handle("/data-semicolon.csv", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "text/csv")
		w.WriteHeader(200)
		w.Write([]byte("foo;bar\nbaz;qux\n"))
	}))
	mux.Handle("/data-colon.csv", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "text/csv")
		w.WriteHeader(200)
		w.Write([]byte("foo:bar\nbaz:qux\n"))
	}))

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
	defer server.Shutdown(t.Context())

	type csvData struct {
		F1 string
		F2 string
	}

	cases := []struct {
		description string
		path        string
		parser      ParserType
	}{
		{"comma", "/data.csv", ParserCSVComma},
		{"semicolon", "/data-semicolon.csv", ParserCSVSemicolon},
		{"colon", "/data-colon.csv", ParserCSVColon},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			r := reporter.NewMock(t)
			config := map[string]Source{
				"local": {
					URL:       fmt.Sprintf("http://%s%s", address, tc.path),
					Method:    "GET",
					Parser:    tc.parser,
					Timeout:   time.Second,
					Interval:  time.Minute,
					Transform: MustParseTransformQuery(".[]"),
				},
			}
			handler := struct {
				data    []csvData
				fetcher *Component[csvData]
			}{}
			handler.fetcher, _ = New[csvData](r, func(ctx context.Context, name string, source Source) (int, error) {
				results, err := handler.fetcher.Fetch(ctx, name, source)
				if err != nil {
					return 0, err
				}
				handler.data = results
				return len(results), nil
			}, "test", config)

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			results, err := handler.fetcher.Fetch(ctx, "local", config["local"])
			if err != nil {
				t.Fatalf("Fetch() error:\n%+v", err)
			}
			expected := []csvData{
				{F1: "foo", F2: "bar"},
				{F1: "baz", F2: "qux"},
			}
			if diff := helpers.Diff(results, expected); diff != "" {
				t.Fatalf("Fetch() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestSourcePlain(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/data.txt", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("hello\nworld\n"))
	}))

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
	defer server.Shutdown(t.Context())

	type plainData struct {
		Name string `validate:"required"`
	}

	r := reporter.NewMock(t)
	config := map[string]Source{
		"local": {
			URL:       fmt.Sprintf("http://%s/data.txt", address),
			Method:    "GET",
			Parser:    ParserPlain,
			Timeout:   time.Second,
			Interval:  time.Minute,
			Transform: MustParseTransformQuery(`[.[] | {name: .}] | .[]`),
		},
	}
	fetcher, _ := New[plainData](r, func(_ context.Context, _ string, _ Source) (int, error) {
		return 0, nil
	}, "test", config)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	results, err := fetcher.Fetch(ctx, "local", config["local"])
	if err != nil {
		t.Fatalf("Fetch() error:\n%+v", err)
	}
	expected := []plainData{
		{Name: "hello"},
		{Name: "world"},
	}
	if diff := helpers.Diff(results, expected); diff != "" {
		t.Fatalf("Fetch() (-got, +want):\n%s", diff)
	}
}

func TestPaginationLinkNext(t *testing.T) {
	mux := http.NewServeMux()
	var address net.Addr
	mux.Handle("/page1.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fmt.Appendf(nil, `{
  "results": [{"name": "item1", "description": "page1"}],
  "next": "http://%s/page2.json"
}`, address))
	}))
	mux.Handle("/page2.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
  "results": [{"name": "item2", "description": "page2"}],
  "next": null
}`))
	}))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	address = listener.Addr()
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Shutdown(t.Context())

	r := reporter.NewMock(t)
	config := map[string]Source{
		"local": {
			URL:        fmt.Sprintf("http://%s/page1.json", address),
			Method:     "GET",
			Timeout:    time.Second,
			Interval:   time.Minute,
			Pagination: PaginationLinkNext,
			Transform:  MustParseTransformQuery(".results[]"),
		},
	}
	fetcher, _ := New[remoteData](r, func(_ context.Context, _ string, _ Source) (int, error) {
		return 0, nil
	}, "test", config)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	results, err := fetcher.Fetch(ctx, "local", config["local"])
	if err != nil {
		t.Fatalf("Fetch() error:\n%+v", err)
	}
	expected := []remoteData{
		{Name: "item1", Description: "page1"},
		{Name: "item2", Description: "page2"},
	}
	if diff := helpers.Diff(results, expected); diff != "" {
		t.Fatalf("Fetch() (-got, +want):\n%s", diff)
	}
}

func TestPaginationRelNext(t *testing.T) {
	mux := http.NewServeMux()
	var address net.Addr
	mux.Handle("/page1.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Link", fmt.Sprintf(`<http://%s/page2.json>; rel="next"`, address))
		w.WriteHeader(200)
		w.Write([]byte(`[{"name": "item1", "description": "page1"}]`))
	}))
	mux.Handle("/page2.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"name": "item2", "description": "page2"}]`))
	}))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	address = listener.Addr()
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Shutdown(t.Context())

	r := reporter.NewMock(t)
	config := map[string]Source{
		"local": {
			URL:        fmt.Sprintf("http://%s/page1.json", address),
			Method:     "GET",
			Timeout:    time.Second,
			Interval:   time.Minute,
			Pagination: PaginationRelNext,
			Transform:  MustParseTransformQuery(".[]"),
		},
	}
	fetcher, _ := New[remoteData](r, func(_ context.Context, _ string, _ Source) (int, error) {
		return 0, nil
	}, "test", config)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	results, err := fetcher.Fetch(ctx, "local", config["local"])
	if err != nil {
		t.Fatalf("Fetch() error:\n%+v", err)
	}
	expected := []remoteData{
		{Name: "item1", Description: "page1"},
		{Name: "item2", Description: "page2"},
	}
	if diff := helpers.Diff(results, expected); diff != "" {
		t.Fatalf("Fetch() (-got, +want):\n%s", diff)
	}
}

func TestPaginationRelNextRelativeURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/api/page1.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Link", `</api/page2.json>; rel="next"`)
		w.WriteHeader(200)
		w.Write([]byte(`[{"name": "item1", "description": "page1"}]`))
	}))
	mux.Handle("/api/page2.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"name": "item2", "description": "page2"}]`))
	}))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Shutdown(t.Context())

	r := reporter.NewMock(t)
	config := map[string]Source{
		"local": {
			URL:        fmt.Sprintf("http://%s/api/page1.json", listener.Addr()),
			Method:     "GET",
			Timeout:    time.Second,
			Interval:   time.Minute,
			Pagination: PaginationRelNext,
			Transform:  MustParseTransformQuery(".[]"),
		},
	}
	fetcher, _ := New[remoteData](r, func(_ context.Context, _ string, _ Source) (int, error) {
		return 0, nil
	}, "test", config)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	results, err := fetcher.Fetch(ctx, "local", config["local"])
	if err != nil {
		t.Fatalf("Fetch() error:\n%+v", err)
	}
	expected := []remoteData{
		{Name: "item1", Description: "page1"},
		{Name: "item2", Description: "page2"},
	}
	if diff := helpers.Diff(results, expected); diff != "" {
		t.Fatalf("Fetch() (-got, +want):\n%s", diff)
	}
}

func TestPaginationAuto(t *testing.T) {
	t.Run("detects rel-next", func(t *testing.T) {
		mux := http.NewServeMux()
		var address net.Addr
		mux.Handle("/page1.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.Header().Add("Link", fmt.Sprintf(`<http://%s/page2.json>; rel="next"`, address))
			w.WriteHeader(200)
			w.Write([]byte(`[{"name": "item1", "description": "page1"}]`))
		}))
		mux.Handle("/page2.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`[{"name": "item2", "description": "page2"}]`))
		}))

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Listen() error:\n%+v", err)
		}
		address = listener.Addr()
		server := &http.Server{Handler: mux}
		go server.Serve(listener)
		defer server.Shutdown(t.Context())

		r := reporter.NewMock(t)
		config := map[string]Source{
			"local": {
				URL:        fmt.Sprintf("http://%s/page1.json", address),
				Method:     "GET",
				Timeout:    time.Second,
				Interval:   time.Minute,
				Pagination: PaginationAuto,
				Transform:  MustParseTransformQuery(".[]"),
			},
		}
		fetcher, _ := New[remoteData](r, func(_ context.Context, _ string, _ Source) (int, error) {
			return 0, nil
		}, "test", config)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		results, err := fetcher.Fetch(ctx, "local", config["local"])
		if err != nil {
			t.Fatalf("Fetch() error:\n%+v", err)
		}
		expected := []remoteData{
			{Name: "item1", Description: "page1"},
			{Name: "item2", Description: "page2"},
		}
		if diff := helpers.Diff(results, expected); diff != "" {
			t.Fatalf("Fetch() (-got, +want):\n%s", diff)
		}
	})

	t.Run("detects link-next", func(t *testing.T) {
		mux := http.NewServeMux()
		var address net.Addr
		mux.Handle("/page1.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write(fmt.Appendf(nil, `{
  "results": [{"name": "item1", "description": "page1"}],
  "next": "http://%s/page2.json"
}`, address))
		}))
		mux.Handle("/page2.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`{
  "results": [{"name": "item2", "description": "page2"}]
}`))
		}))

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Listen() error:\n%+v", err)
		}
		address = listener.Addr()
		server := &http.Server{Handler: mux}
		go server.Serve(listener)
		defer server.Shutdown(t.Context())

		r := reporter.NewMock(t)
		config := map[string]Source{
			"local": {
				URL:        fmt.Sprintf("http://%s/page1.json", address),
				Method:     "GET",
				Timeout:    time.Second,
				Interval:   time.Minute,
				Pagination: PaginationAuto,
				Transform:  MustParseTransformQuery(".results[]"),
			},
		}
		fetcher, _ := New[remoteData](r, func(_ context.Context, _ string, _ Source) (int, error) {
			return 0, nil
		}, "test", config)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		results, err := fetcher.Fetch(ctx, "local", config["local"])
		if err != nil {
			t.Fatalf("Fetch() error:\n%+v", err)
		}
		expected := []remoteData{
			{Name: "item1", Description: "page1"},
			{Name: "item2", Description: "page2"},
		}
		if diff := helpers.Diff(results, expected); diff != "" {
			t.Fatalf("Fetch() (-got, +want):\n%s", diff)
		}
	})

	t.Run("no pagination when not needed", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.Handle("/data.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`[{"name": "item1", "description": "single"}]`))
		}))

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Listen() error:\n%+v", err)
		}
		server := &http.Server{Handler: mux}
		go server.Serve(listener)
		defer server.Shutdown(t.Context())

		r := reporter.NewMock(t)
		config := map[string]Source{
			"local": {
				URL:        fmt.Sprintf("http://%s/data.json", listener.Addr()),
				Method:     "GET",
				Timeout:    time.Second,
				Interval:   time.Minute,
				Pagination: PaginationAuto,
				Transform:  MustParseTransformQuery(".[]"),
			},
		}
		fetcher, _ := New[remoteData](r, func(_ context.Context, _ string, _ Source) (int, error) {
			return 0, nil
		}, "test", config)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		results, err := fetcher.Fetch(ctx, "local", config["local"])
		if err != nil {
			t.Fatalf("Fetch() error:\n%+v", err)
		}
		expected := []remoteData{
			{Name: "item1", Description: "single"},
		}
		if diff := helpers.Diff(results, expected); diff != "" {
			t.Fatalf("Fetch() (-got, +want):\n%s", diff)
		}
	})
}

func TestSourceWithTLS(t *testing.T) {
	cert := generateSelfSignedCert(t)

	// Setup TLS server
	mux := http.NewServeMux()
	mux.Handle("/data.json", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"results": [{"name": "secure", "description": "tls test"}]}`))
	}))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}

	server := &http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	address := listener.Addr().String()
	go server.ServeTLS(listener, "", "")
	defer server.Shutdown(t.Context())

	t.Run("WithoutTLSConfig", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := map[string]Source{
			"secure": {
				URL:       fmt.Sprintf("https://%s/data.json", address),
				Method:    "GET",
				Timeout:   1 * time.Second,
				Interval:  1 * time.Minute,
				Transform: MustParseTransformQuery(".results[]"),
			},
		}
		handler := remoteDataHandler{
			data: []remoteData{},
		}
		handler.fetcher, _ = New[remoteData](r, handler.UpdateData, "test", config)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_, err := handler.fetcher.Fetch(ctx, "secure", config["secure"])
		if err == nil {
			t.Fatal("Fetch() should have errored with certificate error")
		}
	})

	t.Run("WithTLSSkipVerify", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := map[string]Source{
			"secure": {
				URL:      fmt.Sprintf("https://%s/data.json", address),
				Method:   "GET",
				Timeout:  1 * time.Second,
				Interval: 1 * time.Minute,
				TLS: helpers.TLSConfiguration{
					Enable:     true,
					SkipVerify: true,
				},
				Transform: MustParseTransformQuery(".results[]"),
			},
		}
		handler := remoteDataHandler{
			data: []remoteData{},
		}
		handler.fetcher, _ = New[remoteData](r, handler.UpdateData, "test", config)

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		results, err := handler.fetcher.Fetch(ctx, "secure", config["secure"])
		if err != nil {
			t.Fatalf("Fetch() error:\n%+v", err)
		}

		expected := []remoteData{
			{
				Name:        "secure",
				Description: "tls test",
			},
		}
		if diff := helpers.Diff(results, expected); diff != "" {
			t.Fatalf("Fetch() (-got, +want):\n%s", diff)
		}
	})
}
