// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"net/netip"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var diffCmpOptions cmp.Options

// RegisterCmpOption adds an option that will be used in all call to Diff().
func RegisterCmpOption(option cmp.Option) {
	diffCmpOptions = append(diffCmpOptions, option)
}

// Diff return a diff of two objects. If no diff, an empty string is
// returned.
func Diff(a, b any, options ...cmp.Option) string {
	options = append(options, diffCmpOptions...)
	return cmp.Diff(a, b, options...)
}

func init() {
	RegisterCmpOption(cmpopts.EquateComparable(netip.Addr{}))
	RegisterCmpOption(cmpopts.EquateComparable(netip.Prefix{}))
	RegisterCmpOption(cmpopts.EquateErrors())
}
