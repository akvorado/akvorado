// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package remotedatasourcefetcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/go-viper/mapstructure/v2"
	"github.com/itchyny/gojq"
	"gopkg.in/tomb.v2"

	"akvorado/common/reporter"
)

// ProviderFunc is the callback function to call when a datasource is refreshed  implementZ.
type ProviderFunc func(ctx context.Context, name string, source RemoteDataSource) (int, error)

// Component represents a remote data source fetcher.
type Component[T interface{}] struct {
	r           *reporter.Reporter
	t           tomb.Tomb
	provider    ProviderFunc
	dataType    string
	dataSources map[string]RemoteDataSource
	metrics     metrics

	DataSourcesReady chan bool // closed when all data sources are ready
}

// New creates a new remote data source fetcher component.
func New[T interface{}](r *reporter.Reporter, provider ProviderFunc, dataType string, dataSources map[string]RemoteDataSource) (*Component[T], error) {
	c := Component[T]{
		r:                r,
		provider:         provider,
		dataType:         dataType,
		dataSources:      dataSources,
		DataSourcesReady: make(chan bool),
	}
	c.initMetrics()
	return &c, nil
}

// Fetch retrieves data from a configured RemoteDataSource, and returns a list of results
// decoded from JSON to generic type.
// Fetch should be used in UpdateRemoteDataSource implementations to update internal data from results.
func (c *Component[T]) Fetch(ctx context.Context, name string, source RemoteDataSource) ([]T, error) {
	var results []T
	l := c.r.With().Str("name", name).Str("url", source.URL).Logger()
	l.Info().Msg("update data source")

	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}}
	req, err := http.NewRequestWithContext(ctx, source.Method, source.URL, nil)
	for headerName, headerValue := range source.Headers {
		req.Header.Set(headerName, headerValue)
	}
	req.Header.Set("accept", "application/json")
	if err != nil {
		l.Err(err).Msg("unable to build new request")
		return results, fmt.Errorf("unable to build new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		l.Err(err).Msg("unable to fetch data source")
		return results, fmt.Errorf("unable to fetch data source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err := fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, resp.Status)
		l.Error().Msg(err.Error())
		return results, err
	}
	reader := bufio.NewReader(resp.Body)
	decoder := json.NewDecoder(reader)
	var got interface{}
	if err := decoder.Decode(&got); err != nil {
		l.Err(err).Msg("cannot decode JSON output")
		return results, fmt.Errorf("cannot decode JSON output: %w", err)
	}

	iter := source.Transform.Query.RunWithContext(ctx, got)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			l.Err(err).Msg("cannot execute jq filter")
			return results, fmt.Errorf("cannot execute jq filter: %w", err)
		}
		var result T
		config := &mapstructure.DecoderConfig{
			Metadata:   nil,
			Result:     &result,
			DecodeHook: mapstructure.TextUnmarshallerHookFunc(),
		}
		decoder, err := mapstructure.NewDecoder(config)
		if err != nil {
			panic(err)
		}
		if err := decoder.Decode(v); err != nil {
			l.Err(err).Msgf("cannot map returned value for %#v", v)
			return results, fmt.Errorf("cannot map returned value: %w", err)
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		err := errors.New("empty results")
		l.Error().Msg(err.Error())
		return results, err
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

	for name, source := range c.dataSources {
		if source.Transform.Query == nil {
			source.Transform.Query, _ = gojq.Parse(".")
		}

		c.t.Go(func() error {
			c.metrics.remoteDataSourceCount.WithLabelValues(c.dataType, name).Set(0)
			newRetryTicker := func() *backoff.Ticker {
				customBackoff := backoff.NewExponentialBackOff()
				customBackoff.MaxElapsedTime = 0
				customBackoff.MaxInterval = source.Interval
				customBackoff.InitialInterval = source.Interval / 10
				if customBackoff.InitialInterval > time.Second {
					customBackoff.InitialInterval = time.Second
				}
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
