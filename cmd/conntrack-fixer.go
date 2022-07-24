// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

//go:build linux

package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/ti-mo/conntrack"
	"gopkg.in/tomb.v2"

	"akvorado/common/reporter"
)

var conntrackFixerCmd = &cobra.Command{
	Use:   "conntrack-fixer",
	Short: "Clean conntrack for UDP ports",
	Long: `This helper cleans the conntrack entries for the UDP ports exposed by
containers started with the label "akvorado.conntrack.fix=1".`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := reporter.New(reporter.DefaultConfiguration())
		if err != nil {
			return fmt.Errorf("unable to initialize reporter: %w", err)
		}

		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return fmt.Errorf("unable to connect to Docker: %w", err)
		}
		defer cli.Close()

		var t tomb.Tomb
		changes := make(chan bool)
		defer close(changes)

		// Trigger an update
		trigger := func() {
			select {
			case changes <- true:
			case <-t.Dying():
			}
		}

		// Purge conntrack for a given port
		chl, err := conntrack.Dial(nil)
		if err != nil {
			return fmt.Errorf("cannot initialize conntrack support: %w", err)
		}
		defer chl.Close()
		purgeConntrack := func(port uint16) int {
			flows, err := chl.Dump()
			if err != nil {
				r.Err(err).Msg("cannot list conntrack entries")
				return 0
			}
			count := 0
			for _, flow := range flows {
				if flow.TupleOrig.Proto.Protocol == 17 && flow.TupleOrig.Proto.DestinationPort == port {
					count++
					if err := chl.Delete(flow); err != nil {
						r.Err(err).Msg("cannot delete conntrack entry")
					}
				}
			}
			return count
		}

		// Goroutine to watch for changes
		ready := make(chan bool)
		t.Go(func() error {
			filter := filters.NewArgs()
			filter.Add("event", "start")
			filter.Add("label", "akvorado.conntrack.fix=true")
			msgs, errs := cli.Events(t.Context(nil), types.EventsOptions{Filters: filter})
			close(ready)
			for {
				select {
				case <-t.Dying():
					return nil
				case err := <-errs:
					return fmt.Errorf("error while watching for Docker events: %w", err)
				case msg := <-msgs:
					r.Info().
						Str("id", msg.ID).
						Str("from", msg.From).
						Msg("new container started")
					trigger()
				case <-time.After(time.Hour):
					trigger()
				}
			}
		})

		// Goroutine to react to changes
		t.Go(func() error {
			filter := filters.NewArgs()
			filter.Add("label", "akvorado.conntrack.fix=true")
			for {
				select {
				case <-t.Dying():
					return nil
				case <-changes:
					containers, err := cli.ContainerList(t.Context(nil),
						types.ContainerListOptions{
							Filters: filter,
						})
					if err != nil {
						r.Err(err).Msg("cannot list containers")
						continue
					}
					for _, container := range containers {
						details, err := cli.ContainerInspect(t.Context(nil), container.ID)
						if err != nil {
							r.Err(err).Msg("cannot get details on container")
							continue
						}
						for rport, bindings := range details.NetworkSettings.Ports {
							if !strings.HasSuffix(string(rport), "/udp") {
								continue
							}
							ports := map[string]bool{}
							for _, binding := range bindings {
								ports[binding.HostPort] = true
							}
							for hportStr := range ports {
								hport, err := strconv.ParseUint(hportStr, 10, 16)
								if err != nil {
									panic(err)
								}
								l := r.Info().Str("binding",
									fmt.Sprintf("%s -> %d", rport, hport))
								l.Msg("clear conntrack for UDP port")
								if count := purgeConntrack(uint16(hport)); count > 0 {
									l.Msgf("%d entries deleted", count)
								}
							}
						}
					}

				}
			}
		})

		// Trigger now
		<-ready
		r.Info().Msg("conntrack fixer running")
		trigger()

		return t.Wait()
	},
}

func init() {
	RootCmd.AddCommand(conntrackFixerCmd)
}
