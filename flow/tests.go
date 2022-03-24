//go:build !release

package flow

import (
	"testing"

	"akvorado/daemon"
	"akvorado/http"
	"akvorado/reporter"
)

// NewMock creates a new flow importer listening on a random port. It
// is autostarted.
func NewMock(t *testing.T, r *reporter.Reporter, config Configuration) *Component {
	t.Helper()
	config.Listen = "127.0.0.1:0"
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("Start() error:\n%+v", err)
	}
	return c
}

// Inject inject the provided flow message, as if it was received.
func (c *Component) Inject(t *testing.T, fmsg *Message) {
	c.outgoingFlows <- fmsg
}
