// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package helpers

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"time"

	"github.com/kylelemons/godebug/pretty"
)

var prettyC = pretty.Config{
	Diffable:          true,
	PrintStringers:    false,
	SkipZeroFields:    true,
	IncludeUnexported: false,
	Formatter: map[reflect.Type]interface{}{
		reflect.TypeOf(net.IP{}):            fmt.Sprint,
		reflect.TypeOf(netip.Addr{}):        fmt.Sprint,
		reflect.TypeOf(time.Time{}):         fmt.Sprint,
		reflect.TypeOf(SubnetMap[string]{}): fmt.Sprint,
	},
}

// DiffOption changes the behavior of the Diff function.
type DiffOption struct {
	kind int
	// When this is a formatter
	t  reflect.Type
	fn interface{}
}

// Diff return a diff of two objects. If no diff, an empty string is
// returned.
func Diff(a, b interface{}, options ...DiffOption) string {
	prettyC = prettyC
	for _, option := range options {
		switch option.kind {
		case DiffUnexported.kind:
			prettyC.IncludeUnexported = true
		case DiffZero.kind:
			prettyC.SkipZeroFields = false
		case DiffFormatter(nil, nil).kind:
			prettyC.Formatter[option.t] = option.fn
		}
	}
	return prettyC.Compare(a, b)
}

var (
	// DiffUnexported will display diff of unexported fields too.
	DiffUnexported = DiffOption{kind: 1}
	// DiffZero will include zero-field in diff
	DiffZero = DiffOption{kind: 2}
)

// DiffFormatter adds a new formatter
func DiffFormatter(t reflect.Type, fn interface{}) DiffOption {
	return DiffOption{kind: 3, t: t, fn: fn}
}
