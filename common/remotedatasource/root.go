// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasource

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/itchyny/gojq"
	"gopkg.in/tomb.v2"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// ProviderFunc is the callback function to call when a datasource is refreshed.
// The error returned is used for metrics. One should avoid having too many
// different errors.
type ProviderFunc func(ctx context.Context, name string, source Source) (int, error)

// Component represents a remote data source fetcher.
type Component[T any] struct {
	r           *reporter.Reporter
	t           tomb.Tomb
	provider    ProviderFunc
	dataType    string
	dataSources map[string]Source
	metrics     metrics

	DataSourcesReady chan bool // closed when all data sources are ready
}

var (
	// ErrBuildRequest is triggered when we cannot build an HTTP request
	ErrBuildRequest = errors.New("cannot build HTTP request")
	// ErrFetchDataSource is triggered when we cannot fetch the data source
	ErrFetchDataSource = errors.New("cannot fetch data source")
	// ErrStatusCode is triggered if status code is not 200
	ErrStatusCode = errors.New("unexpected HTTP status code")
	// ErrJSONDecode is triggered for any JSON decoding issue
	ErrJSONDecode = errors.New("cannot decode JSON")
	// ErrCSVDecode is triggered for any CSV decoding issue
	ErrCSVDecode = errors.New("cannot decode CSV")
	// ErrMapResult is triggered when we cannot map the result to the expected structure
	ErrMapResult = errors.New("cannot map result")
	// ErrValidate is triggered when there is a check failure
	ErrValidate = errors.New("cannot validate checks")
	// ErrJQExecute is triggered when we cannot execute the jq filter
	ErrJQExecute = errors.New("cannot execute jq filter")
	// ErrEmpty is triggered if the results are empty
	ErrEmpty = errors.New("empty result")
)

// New creates a new remote data source fetcher component.
func New[T any](r *reporter.Reporter, provider ProviderFunc, dataType string, dataSources map[string]Source) (*Component[T], error) {
	c := Component[T]{
		r:                r,
		provider:         provider,
		dataType:         dataType,
		dataSources:      dataSources,
		DataSourcesReady: make(chan bool),
	}

	for k, source := range c.dataSources {
		if source.Transform.Query == nil {
			source.Transform.Query, _ = gojq.Parse(".")
			c.dataSources[k] = source
		}
		if _, err := source.TLS.MakeTLSConfig(); err != nil {
			return nil, err
		}
	}

	c.initMetrics()
	return &c, nil
}

// Fetch retrieves data from a configured Source, and returns a list
// of results decoded from JSON to generic type. Fetch should be used in
// UpdateSource implementations to update internal data from results.
// It outputs errors without details because they are used for metrics.
func (c *Component[T]) Fetch(ctx context.Context, name string, source Source) ([]T, error) {
	var results []T
	l := c.r.With().Str("name", name).Str("url", source.URL).Logger()
	l.Info().Msg("update data source")

	tlsConfig, _ := source.TLS.MakeTLSConfig()
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}}
	req, err := http.NewRequestWithContext(ctx, source.Method, source.URL, nil)
	if err != nil {
		l.Err(err).Msg("unable to build new request")
		return nil, ErrBuildRequest
	}
	for headerName, headerValue := range source.Headers {
		req.Header.Set(headerName, headerValue)
	}
	switch source.Parser {
	case ParserJSON:
		req.Header.Set("accept", "application/json")
	case ParserCSVComma, ParserCSVSemicolon, ParserCSVColon:
		req.Header.Set("accept", "text/csv")
	case ParserPlain:
		req.Header.Set("accept", "text/plain")
	}
	resp, err := client.Do(req)
	if err != nil {
		l.Err(err).Msg("unable to fetch data source")
		return nil, ErrFetchDataSource
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		l.Error().Int("status", resp.StatusCode).Msg("unexpected status code")
		return nil, ErrStatusCode
	}

	got, err := parseResponse(source.Parser, resp.Body)
	if err != nil {
		l.Err(err).Msg("cannot decode response")
		return nil, err
	}

	iter := source.Transform.Query.RunWithContext(ctx, got)
	for idx := 0; ; idx++ {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			l.Err(err).Msg("cannot execute jq filter")
			return nil, ErrJQExecute
		}
		var result T
		config := &mapstructure.DecoderConfig{
			Metadata:   nil,
			Result:     &result,
			DecodeHook: helpers.ProtectedDecodeHookFunc(mapstructure.TextUnmarshallerHookFunc()),
		}
		decoder, err := mapstructure.NewDecoder(config)
		if err != nil {
			panic(err)
		}
		if err := decoder.Decode(v); err != nil {
			l.Err(err).Msg("cannot map returned value")
			return nil, ErrMapResult
		}
		if err := helpers.Validate.StructCtx(ctx, result); err != nil {
			switch err := err.(type) {
			case validator.ValidationErrors:
				l.Err(err).Int("index", idx).Msgf("validation errors on %#v", result)
				return nil, ErrValidate
			default:
				l.Err(err).Int("index", idx).Msgf("unable to validate on %#v", result)
				return nil, ErrValidate
			}
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		l.Error().Msg("empty result")
		return nil, ErrEmpty
	}
	return results, nil
}

// Start the remote data source fetcher component.
func (c *Component[T]) Start() error {
	c.r.Info().Msg("starting remote data source fetcher component")

	var notReadySources sync.WaitGroup
	notReadySources.Add(len(c.dataSources))
	go func() {
		notReadySources.Wait()
		close(c.DataSourcesReady)
	}()
	c.t.Go(func() error {
		// Ensure we have at least one goroutine.
		<-c.t.Dying()
		return nil
	})

	for name, source := range c.dataSources {
		c.t.Go(func() error {
			c.metrics.remoteDataSourceCount.WithLabelValues(c.dataType, name).Set(0)
			newRetryTicker := func() *backoff.Ticker {
				customBackoff := backoff.NewExponentialBackOff()
				customBackoff.MaxInterval = source.Interval
				customBackoff.InitialInterval = source.Interval / 10
				return backoff.NewTicker(customBackoff)
			}
			newRegularTicker := func() *time.Ticker {
				return time.NewTicker(source.Interval)
			}
			retryTicker := newRetryTicker()
			regularTicker := newRegularTicker()
			regularTicker.Stop()
			success := false
			ready := false
			defer func() {
				if !success {
					retryTicker.Stop()
				} else {
					regularTicker.Stop()
				}
				if !ready {
					notReadySources.Done()
				}
			}()
			for {
				ctx, cancel := context.WithTimeout(c.t.Context(nil), source.Timeout)
				count, err := c.provider(ctx, name, source)
				cancel()
				if err == nil {
					c.metrics.remoteDataSourceUpdates.WithLabelValues(c.dataType, name).Inc()
					c.metrics.remoteDataSourceCount.WithLabelValues(c.dataType, name).Set(float64(count))
				} else {
					c.metrics.remoteDataSourceErrors.WithLabelValues(c.dataType, name, err.Error()).Inc()
				}
				if err == nil && !ready {
					ready = true
					notReadySources.Done()
					c.r.Debug().Str("name", name).Msg("source ready")
				}
				if err == nil && !success {
					// On success, change the timer to a regular timer interval
					retryTicker.Stop()
					retryTicker.C = nil
					regularTicker = newRegularTicker()
					success = true
					c.r.Debug().Str("name", name).Msg("switch to regular polling")
				} else if err != nil && success {
					// On failure, switch to the retry ticker
					regularTicker.Stop()
					regularTicker.C = nil
					retryTicker = newRetryTicker()
					success = false
					c.r.Debug().Str("name", name).Msg("switch to retry polling")
				}
				select {
				case <-c.t.Dying():
					return nil
				case <-retryTicker.C:
				case <-regularTicker.C:
				}
			}
		})
	}
	return nil
}

// Stop the remote data source fetcher component.
func (c *Component[T]) Stop() error {
	c.t.Kill(nil)
	return c.t.Wait()
}

// parseResponse parses the HTTP response body according to the parser type.
// It returns a generic value suitable for jq processing.
func parseResponse(parser ParserType, body io.Reader) (any, error) {
	reader := bufio.NewReader(body)
	switch parser {
	case ParserJSON:
		var got any
		if err := json.NewDecoder(reader).Decode(&got); err != nil {
			return nil, ErrJSONDecode
		}
		return got, nil
	case ParserCSVComma, ParserCSVSemicolon, ParserCSVColon:
		csvReader := csv.NewReader(reader)
		switch parser {
		case ParserCSVSemicolon:
			csvReader.Comma = ';'
		case ParserCSVColon:
			csvReader.Comma = ':'
		}
		records, err := csvReader.ReadAll()
		if err != nil {
			return nil, ErrCSVDecode
		}
		result := make([]any, len(records))
		for i, record := range records {
			obj := map[string]any{}
			for j, field := range record {
				obj[fmt.Sprintf("f%d", j+1)] = field
			}
			result[i] = obj
		}
		return result, nil
	case ParserPlain:
		var lines []any
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r")
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return lines, nil
	default:
		return nil, errors.New("unknown parser type")
	}
}
