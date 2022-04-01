package cmd

import (
	"fmt"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
)

// StartStopComponents activate/deactivate components in order.
func StartStopComponents(r *reporter.Reporter, daemonComponent daemon.Component, otherComponents []interface{}) error {
	components := append([]interface{}{r, daemonComponent}, otherComponents...)
	startedComponents := []interface{}{}
	defer func() {
		for _, cmp := range startedComponents {
			if stopperC, ok := cmp.(stopper); ok {
				if err := stopperC.Stop(); err != nil {
					r.Err(err).Msg("unable to stop component, ignoring")
				}
			}
		}
	}()
	for _, cmp := range components {
		if starterC, ok := cmp.(starter); ok {
			if err := starterC.Start(); err != nil {
				return fmt.Errorf("unable to start component: %w", err)
			}
		}
		startedComponents = append([]interface{}{cmp}, startedComponents...)
	}

	r.Info().
		Str("version", Version).Str("build-date", BuildDate).
		Msg("akvorado has started")

	select {
	case <-daemonComponent.Terminated():
		r.Info().Msg("stopping all components")
	}
	return nil
}

type starter interface {
	Start() error
}
type stopper interface {
	Stop() error
}
