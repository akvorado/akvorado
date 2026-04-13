// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package remotedatasource offers a component to refresh internal data
// periodically from a set of remote HTTP sources.
package remotedatasource

import (
	"time"

	"github.com/itchyny/gojq"

	"akvorado/common/helpers"
)

// ParserType defines the format used to parse the response body.
type ParserType int

const (
	// ParserJSON parses the response as JSON.
	ParserJSON ParserType = iota
	// ParserCSVComma parses the response as comma-separated CSV.
	ParserCSVComma
	// ParserCSVSemicolon parses the response as semicolon-separated CSV.
	ParserCSVSemicolon
	// ParserCSVColon parses the response as colon-separated CSV.
	ParserCSVColon
	// ParserPlain parses the response as plain text, one value per line.
	ParserPlain
)

// PaginationType defines the pagination strategy for fetching multiple pages.
type PaginationType int

const (
	// PaginationAuto tries each pagination method until one works, then sticks with it.
	PaginationAuto PaginationType = iota
	// PaginationNone disables pagination.
	PaginationNone
	// PaginationLinkNext finds the next page URL in the JSON body's "next" field.
	PaginationLinkNext
	// PaginationRelNext finds the next page URL in the Link header with rel="next" (RFC 8288).
	PaginationRelNext
)

// Source defines a remote data source.
type Source struct {
	// URL is the URL to fetch to get remote network definition.
	URL string `validate:"url"`
	// Method defines which method to use (GET or POST)
	Method string `validate:"oneof=GET POST"`
	// Headers defines additional headers to send
	Headers map[string]string
	// Proxy is set to true if a proxy should be used.
	Proxy bool
	// Timeout tells the maximum time the remote request should take
	Timeout time.Duration `validate:"min=1s"`
	// Parser defines the format of the response body.
	Parser ParserType
	// Transform is a jq string to transform the received
	// data into a list of attributes.
	Transform TransformQuery
	// Interval tells how much time to wait before updating the source.
	Interval time.Duration `validate:"min=1m"`
	// Pagination defines the pagination strategy for fetching multiple pages.
	Pagination PaginationType
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
	}
}

func init() {
	helpers.RegisterMapstructureUnmarshallerHook(
		helpers.DefaultValuesUnmarshallerHook(DefaultSourceConfiguration()))
}
