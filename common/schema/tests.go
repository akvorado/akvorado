// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package schema

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/google/go-cmp/cmp/cmpopts"
)

var debug = true

// DisableDebug disables debug during the provided test. We keep this as a
// global function for performance reason (when release, debug is a const).
func DisableDebug(t testing.TB) {
	debug = false
	t.Cleanup(func() {
		debug = true
	})
}

// NewMock create a new schema component.
func NewMock(t testing.TB) *Component {
	t.Helper()
	c, err := New(DefaultConfiguration())
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}

// EnableAllColumns enable all columns and returns itself.
func (c *Component) EnableAllColumns() *Component {
	for i := range c.columns {
		c.columns[i].Disabled = false
	}
	c.Schema = c.finalize()
	return c
}

func init() {
	helpers.RegisterCmpOption(cmpopts.IgnoreUnexported(FlowMessage{}))
}
