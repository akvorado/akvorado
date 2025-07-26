// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package kafka

import (
	"context"
	"testing"
	"time"

	"akvorado/common/helpers"
)

func TestMock(t *testing.T) {
	c, incoming := NewMock(t, DefaultConfiguration())

	got := []string{}
	expected := []string{"hello1", "hello2", "hello3"}
	gotAll := make(chan bool)
	shutdownCalled := false
	callback := func(_ context.Context, message []byte) error {
		got = append(got, string(message))
		if len(got) == len(expected) {
			close(gotAll)
		}
		return nil
	}
	c.StartWorkers(
		func(_ int) (ReceiveFunc, ShutdownFunc) {
			return callback, func() { shutdownCalled = true }
		},
	)

	// Produce messages and wait for them
	for _, msg := range expected {
		incoming <- []byte(msg)
	}
	select {
	case <-time.After(time.Second):
		t.Fatal("Too long to get messages")
	case <-gotAll:
	}

	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("Didn't received the expected messages (-got, +want):\n%s", diff)
	}

	c.Stop()
	if !shutdownCalled {
		t.Error("Stop() should have triggered shutdown function")
	}
}
