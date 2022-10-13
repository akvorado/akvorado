// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/mitchellh/mapstructure"
)

type externalNetworkAttributes struct {
	Prefix            netip.Prefix
	NetworkAttributes `mapstructure:",squash"`
}

// updateNetworkSource updates a remote network source. It returns the
// number of networks retrieved.
func (c *Component) updateNetworkSource(ctx context.Context, name string, source NetworkSource) (int, error) {
	l := c.r.With().Str("name", name).Str("url", source.URL).Logger()
	l.Info().Msg("update network source")

	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}}
	req, err := http.NewRequestWithContext(ctx, "GET", source.URL, nil)
	req.Header.Set("accept", "application/json")
	if err != nil {
		l.Err(err).Msg("unable to build new request")
		return 0, fmt.Errorf("unable to build new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		l.Err(err).Msg("unable to fetch network source")
		return 0, fmt.Errorf("unable to fetch network source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err := fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, resp.Status)
		l.Error().Msg(err.Error())
		return 0, err
	}
	reader := bufio.NewReader(resp.Body)
	decoder := json.NewDecoder(reader)
	var got interface{}
	if err := decoder.Decode(&got); err != nil {
		l.Err(err).Msg("cannot decode JSON output")
		return 0, fmt.Errorf("cannot decode JSON output: %w", err)
	}
	results := []externalNetworkAttributes{}
	iter := source.Transform.Query.RunWithContext(ctx, got)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			l.Err(err).Msg("cannot execute jq filter")
			return 0, fmt.Errorf("cannot execute jq filter: %w", err)
		}
		var result externalNetworkAttributes
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
			return 0, fmt.Errorf("cannot map returned value: %w", err)
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		err := errors.New("empty results")
		l.Error().Msg(err.Error())
		return 0, err
	}
	c.networkSourcesLock.Lock()
	c.networkSources[name] = results
	c.networkSourcesLock.Unlock()
	return len(results), nil
}
