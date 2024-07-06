// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

// Package conntrackfixer remove conntrack entries from selected containers
package conntrackfixer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"akvorado/common/daemon"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/ti-mo/conntrack"
	"gopkg.in/tomb.v2"
)

// Component represents the broker.
type Component struct {
	r *reporter.Reporter
	d *Dependencies
	t tomb.Tomb

	dockerClient  DockerClient
	conntrackConn ConntrackConn

	changes chan bool
	healthy chan reporter.ChannelHealthcheckFunc

	metrics struct {
		conntrackDeleted *reporter.CounterVec
		runs             *reporter.CounterVec
		errors           *reporter.CounterVec
	}
}

// Dependencies define the dependencies of the broker.
type Dependencies struct {
	HTTP   *httpserver.Component
	Daemon daemon.Component
}

// New creates a new component
func New(r *reporter.Reporter, dependencies Dependencies) (*Component, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize Docker client: %w", err)
	}
	cli.NegotiateAPIVersion(context.Background())
	chl, err := conntrack.Dial(nil)
	if err != nil {
		cli.Close()
		return nil, fmt.Errorf("cannot initialize conntrack support: %w", err)
	}

	c := Component{
		r: r,
		d: &dependencies,

		dockerClient:  cli,
		conntrackConn: chl,

		changes: make(chan bool),
		healthy: make(chan reporter.ChannelHealthcheckFunc),
	}

	c.metrics.conntrackDeleted = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "conntrack_deleted_total",
			Help: "Number of conntrack entries deleted.",
		},
		[]string{"container", "port"},
	)
	c.metrics.runs = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "runs_total",
			Help: "Number of conntrack cleaning runs triggered.",
		},
		[]string{"reason"},
	)
	c.metrics.errors = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "errors_total",
			Help: "Number of non-fatal errors.",
		},
		[]string{"error"},
	)

	c.d.Daemon.Track(&c.t, "conntrack-fixer")
	return &c, nil
}

// Start the conntrack fixer component
func (c *Component) Start() error {
	c.r.Info().Msg("starting conntrack-fixer component")
	c.r.RegisterHealthcheck("conntrack-fixer", c.channelHealthcheck())

	// Trigger an update
	trigger := func() {
		select {
		case c.changes <- true:
		case <-c.t.Dying():
		}
	}

	// Goroutine to watch for changes
	ready := make(chan bool)
	c.t.Go(func() error {
		filter := filters.NewArgs()
		filter.Add("event", "start")
		filter.Add("label", "akvorado.conntrack.fix=true")
		msgs, errs := c.dockerClient.Events(c.t.Context(nil), types.EventsOptions{Filters: filter})
		close(ready)
		for {
			t := time.NewTimer(5 * time.Minute)
			select {
			case <-c.t.Dying():
				return nil
			case err := <-errs:
				return fmt.Errorf("error while watching for Docker events: %w", err)
			case msg := <-msgs:
				c.r.Info().
					Str("id", msg.ID).
					Str("from", msg.From).
					Msg("new container started")
				c.metrics.runs.WithLabelValues("new container").Inc()
				trigger()
			case <-t.C:
				c.metrics.runs.WithLabelValues("schedule").Inc()
				trigger()
			}
			t.Stop()
		}
	})

	// Goroutine to react to changes
	c.t.Go(func() error {
		filter := filters.NewArgs()
		filter.Add("label", "akvorado.conntrack.fix=true")
		for {
			select {
			case <-c.t.Dying():
				return nil
			case cb, ok := <-c.healthy:
				if ok {
					ctx, cancel := context.WithTimeout(c.t.Context(nil), time.Second)
					if _, err := c.dockerClient.Ping(ctx); err == nil {
						cb(reporter.HealthcheckOK, "docker client alive")
					} else {
						cb(reporter.HealthcheckWarning, "docker client unavailable")
					}
					cancel()
				}
			case <-c.changes:
				containers, err := c.dockerClient.ContainerList(c.t.Context(nil),
					container.ListOptions{
						Filters: filter,
					})
				if err != nil {
					c.r.Err(err).Msg("cannot list containers")
					c.metrics.errors.WithLabelValues("cannot list containers").Inc()
					continue
				}
				for _, container := range containers {
					details, err := c.dockerClient.ContainerInspect(c.t.Context(nil), container.ID)
					if err != nil {
						c.r.Err(err).Msg("cannot get details on container")
						c.metrics.errors.WithLabelValues("cannot get details on container").Inc()
						continue
					}
					for rport, bindings := range details.NetworkSettings.Ports {
						if !strings.HasSuffix(string(rport), "/udp") {
							continue
						}
						ports := map[string]struct{}{}
						for _, binding := range bindings {
							ports[binding.HostPort] = struct{}{}
						}
						for hportStr := range ports {
							hport, err := strconv.ParseUint(hportStr, 10, 16)
							if err != nil {
								panic(err)
							}
							l := c.r.With().Str("binding",
								fmt.Sprintf("%s -> %d", rport, hport)).Logger()
							l.Info().Msg("clear conntrack for UDP port")
							if count := c.purgeConntrack(uint16(hport)); count > 0 {
								c.metrics.conntrackDeleted.
									WithLabelValues(container.ID, hportStr).
									Add(float64(count))
								l.Info().Msgf("%d entries deleted", count)
							}
						}
					}
				}

			}
		}
	})

	// Trigger now
	<-ready
	c.r.Info().Msg("conntrack fixer running")
	c.metrics.runs.WithLabelValues("start").Inc()
	trigger()

	return nil
}

// Stop stops the conntrack-fixer component
func (c *Component) Stop() error {
	c.r.Info().Msg("stopping conntrack-fixer component")
	defer func() {
		close(c.changes)
		c.conntrackConn.Close()
		c.dockerClient.Close()
		c.r.Info().Msg("conntrack-fixer component stopped")
	}()
	c.t.Kill(nil)
	return c.t.Wait()
}

func (c *Component) channelHealthcheck() reporter.HealthcheckFunc {
	return reporter.ChannelHealthcheck(c.t.Context(nil), c.healthy)
}

// purgeConntrack purge the conntrack for the given port.
func (c *Component) purgeConntrack(port uint16) int {
	flows, err := c.conntrackConn.Dump(nil)
	if err != nil {
		c.r.Err(err).Msg("cannot list conntrack entries")
		c.metrics.errors.WithLabelValues("cannot list conntrack entries").Inc()
		return 0
	}
	count := 0
	for _, flow := range flows {
		if flow.TupleOrig.Proto.Protocol == 17 && flow.TupleOrig.Proto.DestinationPort == port {
			if err := c.conntrackConn.Delete(flow); err != nil {
				c.r.Err(err).Msg("cannot delete conntrack entry")
				c.metrics.errors.WithLabelValues("cannot delete conntrack entries").Inc()
			} else {
				count++
			}
		}
	}
	return count
}
