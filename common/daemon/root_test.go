// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package daemon

import (
	"errors"
	"testing"
	"testing/synctest"

	"gopkg.in/tomb.v2"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func TestTerminate(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	select {
	case <-c.Terminated():
		t.Fatalf("Terminated() was closed while we didn't request termination")
	default:
		// OK
	}

	c.Terminate()
	select {
	case _, ok := <-c.Terminated():
		if ok {
			t.Fatalf("Terminated() returned an unexpected value")
		}
		// OK
	default:
		t.Fatalf("Terminated() wasn't closed while we requested it to be")
	}

	c.Terminate() // Can be called several times.
}

func TestStop(t *testing.T) {
	r := reporter.NewMock(t)
	c, err := New(r)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	c.Start()

	select {
	case <-c.Terminated():
		t.Fatalf("Terminated() was closed while we didn't request termination")
	default:
		// OK
	}

	c.Stop()
	select {
	case _, ok := <-c.Terminated():
		if ok {
			t.Fatalf("Terminated() returned an unexpected value")
		}
		// OK
	default:
		t.Fatalf("Terminated() wasn't closed while we requested it to be")
	}
}

func TestTombTracking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var tomb tomb.Tomb
		r := reporter.NewMock(t)
		c, err := New(r)
		if err != nil {
			t.Fatalf("New() error:\n%+v", err)
		}

		c.Track(&tomb, "tomb")
		helpers.StartStop(t, c)

		ch := make(chan bool)
		tomb.Go(func() error {
			select {
			case <-tomb.Dying():
				t.Fatalf("Dying() should not happen inside the tomb")
			case <-ch:
				return errors.New("crashing")
			}
			return nil
		})
		synctest.Wait()

		select {
		case <-tomb.Dying():
			t.Fatalf("Dying() was closed while the tomb is not dead")
		case <-c.Terminated():
			t.Fatalf("Terminated() was closed while we didn't request termination")
		default:
			// OK
		}

		close(ch)
		tomb.Wait()
		synctest.Wait()
		select {
		case <-c.Terminated():
			// OK
		default:
			t.Fatalf("Terminated() was not closed while tomb is dead")
		}

		c.Stop()
	})
}
