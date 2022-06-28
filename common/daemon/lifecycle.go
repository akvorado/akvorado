// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package daemon

import (
	"sync"
)

// lifecycleComponent is the lifecycle part of a component.
type lifecycleComponent struct {
	terminateChannel chan struct{}
	terminateOnce    sync.Once
}

// Terminated will return a channel that will be closed when the daemon
// needs to terminate.
func (c *lifecycleComponent) Terminated() <-chan struct{} {
	return c.terminateChannel
}

// Terminate should be called to request termination of a daemon.
func (c *lifecycleComponent) Terminate() {
	c.terminateOnce.Do(func() { close(c.terminateChannel) })
}
