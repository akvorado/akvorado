// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"context"
	"net"
	"time"

	"github.com/cenkalti/backoff/v7"
)

// Accept waits for the next connection on the listener. When Accept() fails,
// notify is called and Accept() is tried again after a delay with backoff.
func Accept(ctx context.Context, listener net.Listener, notify func(err error, delay time.Duration)) (net.Conn, error) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 5 * time.Millisecond
	b.Multiplier = 2
	b.MaxInterval = time.Second
	b.RandomizationFactor = 0
	return backoff.Retry(ctx, listener.Accept,
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(0),
		backoff.WithNotify(notify))
}
