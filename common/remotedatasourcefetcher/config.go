// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package remotedatasourcefetcher offers a component to refresh internal data periodically
// from a set of remote HTTP sources in JSON format.
package remotedatasourcefetcher

import (
	"time"

	"github.com/itchyny/gojq"

	"akvorado/common/helpers"
)

// RemoteDataSource defines a remote network definition.
type RemoteDataSource struct {
	// URL is the URL to fetch to get remote network definition.
	// It should provide a JSON file.
	URL string `validate:"url"`
	// Method defines which method to use (GET or POST)
	Method string `validate:"oneof=GET POST"`
	// Headers defines additional headers to send
	Headers map[string]string
	// Proxy is set to true if a proxy should be used.
	Proxy bool
	// Timeout tells the maximum time the remote request should take
	Timeout time.Duration `validate:"min=1s"`
	// Transform is a jq string to transform the received JSON
	// data into a list of network attributes.
	Transform TransformQuery
	// Interval tells how much time to wait before updating the source.
	Interval time.Duration `validate:"min=1m"`
}

// TransformQuery represents a jq query to transform data.
type TransformQuery struct {
	*gojq.Query
}

// UnmarshalText parses a jq query.
func (jq *TransformQuery) UnmarshalText(text []byte) error {
	q, err := gojq.Parse(string(text))
	if err != nil {
		return err
	}
	*jq = TransformQuery{q}
	return nil
}

// String turns a jq query into a string.
func (jq TransformQuery) String() string {
	if jq.Query != nil {
		return jq.Query.String()
	}
	return ".[]"
}

// MarshalText turns a jq query into a bytearray.
func (jq TransformQuery) MarshalText() ([]byte, error) {
	return []byte(jq.String()), nil
}

// DefaultRemoteDataSourceConfiguration is the default configuration for a network source.
func DefaultRemoteDataSourceConfiguration() RemoteDataSource {
	return RemoteDataSource{
		Method:  "GET",
		Timeout: time.Minute,
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultRemoteDataSourceConfiguration()))
}
