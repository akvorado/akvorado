// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package conntrackfixer

import (
	"github.com/docker/docker/client"
	"github.com/ti-mo/conntrack"
)

// ConntrackConn is the part of conntrack.Conn we use
type ConntrackConn interface {
	Close() error
	Dump() ([]conntrack.Flow, error)
	Delete(f conntrack.Flow) error
}

// DockerClient is the part of docker.Client we use
type DockerClient interface {
	client.ContainerAPIClient
	client.SystemAPIClient
	Close() error
}
