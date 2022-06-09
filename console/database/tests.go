//go:build !release

package database

import (
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

// NewMock instantiantes a new authentication component
func NewMock(t *testing.T, r *reporter.Reporter) *Component {
	t.Helper()
	c, err := New(r, DefaultConfiguration())
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
}
