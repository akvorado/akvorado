// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package flow

import (
	"testing"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/http"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/inlet/flow/input/udp"
)

// NewMock creates a new flow importer listening on a random port. It
// is autostarted.
func NewMock(t *testing.T, r *reporter.Reporter, config Configuration) *Component {
	t.Helper()
	if config.Inputs == nil {
		config.Inputs = []InputConfiguration{
			{
				Decoder: "netflow",
				Config: &udp.Configuration{
					Listen:    "127.0.0.1:0",
					QueueSize: 10,
				},
			},
		}
	}
	c, err := New(r, config, Dependencies{
		Daemon: daemon.NewMock(t),
		HTTP:   http.NewMock(t, r),
		Schema: schema.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)
	return c
}

// Inject inject the provided flow message, as if it was received.
func (c *Component) Inject(fmsg *schema.FlowMessage) {
	c.outgoingFlows <- fmsg
}
