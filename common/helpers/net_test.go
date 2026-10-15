// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

// failingListener fails a given number of times before returning a connection.
type failingListener struct {
	net.Listener
	failures int
	conn     net.Conn
}

func (l *failingListener) Accept() (net.Conn, error) {
	if l.failures > 0 {
		l.failures--
		return nil, syscall.EMFILE
	}
	return l.conn, nil
}

func TestAccept(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()
		listener := &failingListener{failures: 10, conn: server}
		delays := []time.Duration{}
		start := time.Now()
		conn, err := Accept(t.Context(), listener, func(err error, delay time.Duration) {
			if !errors.Is(err, syscall.EMFILE) {
				t.Errorf("Accept() error:\n%+v", err)
			}
			delays = append(delays, delay)
		})
		if err != nil {
			t.Fatalf("Accept() error:\n%+v", err)
		}
		if conn != server {
			t.Error("Accept() did not return the expected connection")
		}
		expected := []time.Duration{
			5 * time.Millisecond,
			10 * time.Millisecond,
			20 * time.Millisecond,
			40 * time.Millisecond,
			80 * time.Millisecond,
			160 * time.Millisecond,
			320 * time.Millisecond,
			640 * time.Millisecond,
			time.Second,
			time.Second,
		}
		if diff := Diff(delays, expected); diff != "" {
			t.Errorf("Accept() delays (-got, +want):\n%s", diff)
		}
		if diff := Diff(time.Since(start), 3275*time.Millisecond); diff != "" {
			t.Errorf("Accept() duration (-got, +want):\n%s", diff)
		}
	})
}

func TestAcceptCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := &failingListener{failures: 1000}
		start := time.Now()
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		_, err := Accept(ctx, listener, func(error, time.Duration) {})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Accept() error:\n%+v", err)
		}
		if diff := Diff(time.Since(start), 10*time.Second); diff != "" {
			t.Errorf("Accept() duration (-got, +want):\n%s", diff)
		}
	})
}
