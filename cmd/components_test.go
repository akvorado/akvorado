// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd_test

import (
	"errors"
	"testing"

	"akvorado/cmd"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

type Startable struct {
	Started bool
}
type Stopable struct {
	Stopped bool
}

func (c *Startable) Start() error {
	c.Started = true
	return nil
}
func (c *Stopable) Stop() error {
	c.Stopped = true
	return nil
}

type ComponentStartStop struct {
	Startable
	Stopable
}
type ComponentStop struct {
	Stopable
}
type ComponentStart struct {
	Startable
}
type ComponentNone struct{}
type ComponentStartError struct {
	Stopable
}

func (c ComponentStartError) Start() error {
	return errors.New("nooo")
}

func TestStartStop(t *testing.T) {
	r := reporter.NewMock(t)
	daemonComponent := daemon.NewMock(t)
	otherComponents := []interface{}{
		&ComponentStartStop{},
		&ComponentStop{},
		&ComponentStart{},
		&ComponentNone{},
		&ComponentStartError{},
		&ComponentStartStop{},
	}
	if err := cmd.StartStopComponents(r, daemonComponent, otherComponents); err == nil {
		t.Error("StartStopComponents() did not trigger an error")
	}

	expected := []interface{}{
		&ComponentStartStop{
			Startable: Startable{Started: true},
			Stopable:  Stopable{Stopped: true},
		},
		&ComponentStop{
			Stopable: Stopable{Stopped: true},
		},
		&ComponentStart{
			Startable: Startable{Started: true},
		},
		&ComponentNone{},
		&ComponentStartError{},
		&ComponentStartStop{},
	}
	if diff := helpers.Diff(otherComponents, expected); diff != "" {
		t.Errorf("StartStopComponents() (-got, +want):\n%s", diff)
	}
}
