// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build !release

package daemon

import (
	"testing"

	"gopkg.in/tomb.v2"
)

// MockComponent is a daemon component that does nothing. It doesn't
// need to be started to work.
type MockComponent struct {
	lifecycleComponent
}

// NewMock will create a daemon component that does nothing.
func NewMock(t *testing.T) Component {
	t.Helper()
	return &MockComponent{
		lifecycleComponent: lifecycleComponent{
			terminateChannel: make(chan struct{}),
		},
	}
}

// Start does nothing.
func (c *MockComponent) Start() error {
	return nil
}

// Stop does nothing.
func (c *MockComponent) Stop() error {
	c.Terminate()
	return nil
}

// Track does nothing
func (c *MockComponent) Track(t *tomb.Tomb, who string) {
	return
}
