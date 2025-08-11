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
}

func formatByte(v any) string {
	return fmt.Sprintf("0x%x", v)
}

func defaultPrettyFormatters() map[reflect.Type]any {
	result := map[reflect.Type]any{
		reflect.TypeOf(net.IP{}):            fmt.Sprint,
		reflect.TypeOf(netip.Addr{}):        fmt.Sprint,
		reflect.TypeOf(netip.Prefix{}):      fmt.Sprint,
		reflect.TypeOf(time.Time{}):         fmt.Sprint,
		reflect.TypeOf(SubnetMap[string]{}): fmt.Sprint,
		reflect.TypeOf(SubnetMap[uint]{}):   fmt.Sprint,
		reflect.TypeOf(SubnetMap[uint16]{}): fmt.Sprint,
		reflect.TypeOf(byte(0)):             formatByte,
	}
	for t, fn := range nonDefaultPrettyFormatters {
		result[t] = fn
	}
	return result
}

var nonDefaultPrettyFormatters = map[reflect.Type]any{}

// AddPrettyFormatter add a global pretty formatter. We cannot put everything in
// the default map due to cycles.
func AddPrettyFormatter(t reflect.Type, fn any) {
	nonDefaultPrettyFormatters[t] = fn
}

// DiffOption changes the behavior of the Diff function.
type DiffOption struct {
	kind int
	// When this is a formatter
	t  reflect.Type
	fn any
}

// Diff return a diff of two objects. If no diff, an empty string is
// returned.
func Diff(a, b any, options ...DiffOption) string {
	prettyC := prettyC
	prettyC.Formatter = defaultPrettyFormatters()
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
func DiffFormatter(t reflect.Type, fn any) DiffOption {
	return DiffOption{kind: 3, t: t, fn: fn}
}
