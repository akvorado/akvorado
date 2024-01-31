// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package remotedatasourcefetcher

import "github.com/itchyny/gojq"

// MustParseTransformQuery parses a transform query or panic.
func MustParseTransformQuery(src string) TransformQuery {
	q, err := gojq.Parse(src)
	if err != nil {
		panic(err)
	}
	return TransformQuery{q}
}
