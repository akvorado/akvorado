// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package remotedatasource offers a component to refresh internal data
// periodically from a set of remote HTTP sources in JSON format.
package remotedatasource

import (
	"time"

	"github.com/itchyny/gojq"

	"akvorado/common/helpers"
)

// Source defines a remote data source.
type Source struct {
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
	// TLS defines the TLS configuration if the URL needs it.
	TLS helpers.TLSConfiguration
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

// DefaultSourceConfiguration is the default configuration for a network source.
func DefaultSourceConfiguration() Source {
	return Source{
		Method:  "GET",
		Timeout: time.Minute,
		TLS: helpers.TLSConfiguration{
			Enable: false,
			Verify: true,
		},
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultSourceConfiguration()))
}
