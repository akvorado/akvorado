// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package gnmi

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"akvorado/outlet/metadata/provider"

	"github.com/cenkalti/backoff/v4"
	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/target"
)

// exporterState is the state of an exporter.
type exporterState struct {
	Name       string
	ready      bool      // ready for the first time
	Ready      chan bool // not polling, data ready
	Interfaces map[uint]provider.Interface
}

// update update a state with the received events.
func (state *exporterState) update(events []event, model Model) {
	// First pass:
	// - system name
	// - mapping from keys to indexes (we assume this works, this may not be the
	// case on systems where the same key can be used for two distinct
	// interfaces, but our model can't really link an index to the other
	// properties, for example if the index is in the state hierarchy but the
	// name is in the config hierarchy)
	// - mapping from keys to speeds (same remark)
	i := 0
	indexes := map[string]uint{}
	speeds := map[string]uint{}
outer1:
	for _, event := range events {
		for _, path := range model.SystemNamePaths {
			if event.Path == path {
				state.Name = event.Value
				continue outer1
			}
		}
		for _, path := range model.IfIndexPaths {
			if event.Path == path {
				index, err := strconv.ParseUint(event.Value, 10, 32)
				if err != nil {
					continue outer1
				}
				indexes[event.Keys] = uint(index)
			}
		}
		for _, path := range model.IfSpeedPaths {
			if event.Path == path.Path {
				speed, err := convertSpeed(event.Value, path.Unit)
				if err != nil {
					continue outer1
				}
				speeds[event.Keys] = speed
			}
		}
		events[i] = event
		i++
	}
	events = events[:i]

	// Second pass: names and descriptions
	state.Interfaces = map[uint]provider.Interface{}
outer2:
	for _, event := range events {
		for _, path := range model.IfNamePaths {
			if event.Path == path {
				keys := event.Keys
				index, ok := indexes[keys]
				if !ok {
					continue
				}
				iface := state.Interfaces[index]
				iface.Name = event.Value
				state.Interfaces[index] = iface
				continue outer2
			}
		}
		for _, path := range model.IfDescriptionPaths {
			if event.Path == path {
				index, ok := indexes[event.Keys]
				if !ok {
					continue
				}
				iface := state.Interfaces[index]
				iface.Description = event.Value
				state.Interfaces[index] = iface
				continue outer2
			}
		}
	}

	// Third-pass: unnamed interfaces and speed
	for keys, index := range indexes {
		iface := state.Interfaces[index]
		// Set name
		if iface.Name == "" && len(model.IfNameKeys) > 0 {
		inner3:
			for _, key := range strings.Split(keys, ",") {
				for _, name := range model.IfNameKeys {
					pfx := fmt.Sprintf("%s=", name)
					if strings.HasPrefix(key, pfx) {
						iface.Name = key[len(pfx):]
						break inner3
					}
				}
			}
		}
		// When no name, delete the interface
		if iface.Name == "" {
			delete(state.Interfaces, index)
			continue
		}
		// Set speed
		for iface.Speed == 0 && keys != "" {
			iface.Speed = speeds[keys]
			keys = keys[:max(0, strings.LastIndex(keys, ","))]
		}
		// Copy back
		state.Interfaces[index] = iface
	}
}

// startCollector starts a new gNMI collector with the given state. It should not be used with taking the lock.
func (p *Provider) startCollector(ctx context.Context, exporterIP netip.Addr, state *exporterState) {
	exporterStr := exporterIP.Unmap().String()
	l := p.r.With().Str("exporter", exporterStr).Logger()
	p.metrics.ready.WithLabelValues(exporterStr).Set(0)
	retryInitBackoff := backoff.NewExponentialBackOff()
	retryInitBackoff.MaxElapsedTime = 0
	retryInitBackoff.MaxInterval = 5 * time.Minute
	retryInitBackoff.InitialInterval = time.Second
	l.Info().Msg("starting gNMI collector")
	defer l.Info().Msg("stopping gNMI collector")
retryConnect:
	targetIP, ok := p.config.Targets.Lookup(exporterIP)
	if !ok {
		targetIP = exporterIP
	}
	targetPort := p.config.Ports.LookupOrDefault(exporterIP, 57400)
	targetAddress := net.JoinHostPort(targetIP.String(), strconv.FormatUint(uint64(targetPort), 10))
	targetAuthParameters := p.config.AuthenticationParameters.LookupOrDefault(exporterIP, AuthenticationParameter{})
	targetOptions := []api.TargetOption{
		api.Address(targetAddress),
		api.Insecure(targetAuthParameters.Insecure),
		api.SkipVerify(targetAuthParameters.SkipVerify),
		api.Timeout(p.config.Timeout),
	}
	addIfNotEmpty := func(param string, option api.TargetOption) {
		if param != "" {
			targetOptions = append(targetOptions, option)
		}
	}
	addIfNotEmpty(targetAuthParameters.Username, api.Username(targetAuthParameters.Username))
	addIfNotEmpty(targetAuthParameters.Password, api.Password(targetAuthParameters.Password))
	addIfNotEmpty(targetAuthParameters.TLSCA, api.TLSCA(targetAuthParameters.TLSCA))
	addIfNotEmpty(targetAuthParameters.TLSCert, api.TLSCert(targetAuthParameters.TLSCert))
	addIfNotEmpty(targetAuthParameters.TLSKey, api.TLSKey(targetAuthParameters.TLSKey))

	waitBeforeRetry := func() bool {
		next := time.NewTimer(retryInitBackoff.NextBackOff())
		select {
		case <-ctx.Done():
			next.Stop()
			return false
		case <-next.C:
		}
		return true
	}
	l.Debug().Msgf("connecting to %s", targetAddress)
	tg, err := api.NewTarget(
		targetOptions...,
	)
	if err != nil {
		l.Err(err).Msg("unable to create target")
		p.metrics.errors.WithLabelValues(exporterStr, "cannot create target").Inc()
		if !waitBeforeRetry() {
			return
		}
		goto retryConnect
	}

	err = tg.CreateGNMIClient(ctx)
	if err != nil {
		l.Err(err).Msg("unable to create client")
		p.metrics.errors.WithLabelValues(exporterStr, "cannot create client").Inc()
		if !waitBeforeRetry() {
			return
		}
		goto retryConnect
	}
	defer tg.Close()

retryDetect:
	// We need to detect the model
	model, encoding, err := p.detectModelAndEncoding(ctx, tg)
	if err != nil {
		l.Err(err).Msg("unable to detect model")
		p.metrics.errors.WithLabelValues(exporterStr, "cannot detect model").Inc()
		if !waitBeforeRetry() {
			return
		}
		goto retryDetect
	}
	l.Info().Msgf("model is %q, encoding is %q", model.Name, encoding)
	p.metrics.models.WithLabelValues(exporterStr, model.Name).Set(1)
	p.metrics.encodings.WithLabelValues(exporterStr, encoding).Set(1)

	// Receive updates. There are several possibilities:
	// - SubscribeOnce: works as expected, but needs polling
	// - Subscribe, mode stream + on change: no deletes received, some implementations may not send changes
	// - Subscribe, mode stream + sampling: we cannot know when stuff get deleted without expiring them ourselves
	// - SubscribePoll: not widely implemented
	//
	// So, we use SubscribeOnce. This is not the most efficient way, but we ensure we get a coherent state.
	subscribeRequestOptions := model.gnmiOptions(
		api.SubscriptionListModeONCE(),
		api.Encoding(encoding),
	)
	if setTarget, ok := p.config.SetTarget.Lookup(exporterIP); ok && setTarget {
		subscribeRequestOptions = append(subscribeRequestOptions, api.Target(exporterStr))
	}
	subscribeReq, err := api.NewSubscribeRequest(subscribeRequestOptions...)
	if err != nil {
		panic(fmt.Errorf("NewSubscribeRequest() error: %w", err))
	}
	retryFetchBackoff := backoff.NewExponentialBackOff()
	retryFetchBackoff.MaxElapsedTime = 0
	retryFetchBackoff.MaxInterval = time.Minute
	retryFetchBackoff.InitialInterval = time.Second
	for {
		l.Debug().Msg("polling")
		start := time.Now()
		subscribeResp, err := tg.SubscribeOnce(ctx, subscribeReq)
		p.metrics.times.WithLabelValues(exporterStr).Observe(time.Since(start).Seconds())
		if err == nil {
			events := subscribeResponsesToEvents(subscribeResp)
			p.metrics.paths.WithLabelValues(exporterStr).Set(float64(len(events)))
			p.stateLock.Lock()
			state.update(events, model)
			state.ready = true
			p.stateLock.Unlock()
			l.Debug().Msg("state updated")
			p.metrics.ready.WithLabelValues(exporterStr).Set(1)
			p.metrics.updates.WithLabelValues(exporterStr).Inc()

			// In the following window, we consider ourselves ready and unlock
			// waiting clients to check for data.

			// On success, wait a bit before next refresh interval and ignore
			// any refresh requests.
			next := time.NewTimer(p.config.MinimalRefreshInterval)
		outerWaitRefreshTimer:
			for {
				select {
				case state.Ready <- true:
				case <-ctx.Done():
					next.Stop()
					return
				case <-p.refresh:
				case <-next.C:
					break outerWaitRefreshTimer
				}
			}
			// Wait for a new message in refresh queue
			l.Debug().Msg("wait for refresh request")
		outerWaitRefresh:
			for {
				select {
				case state.Ready <- true:
				case <-ctx.Done():
					return
				case <-p.refresh:
					break outerWaitRefresh
				}
			}
			// Reset retry timer and do the next fresh
			retryFetchBackoff.Reset()
		} else {
			// On error, retry a bit later. While retrying, if we have an
			// initial state, consider ourselves ready.
			l.Err(err).Msg("cannot poll")
			p.metrics.errors.WithLabelValues(exporterStr, "cannot poll").Inc()
			next := time.NewTimer(retryFetchBackoff.NextBackOff())
			var readyChan chan bool
			if state.ready {
				readyChan = state.Ready
			}
		outerWaitRetryTimer:
			for {
				select {
				case readyChan <- true:
				case <-ctx.Done():
					next.Stop()
					return
				case <-next.C:
					break outerWaitRetryTimer
				}
			}
		}
	}
}

// detectModelAndEncoding subscribe to the various paths of the configured models to
// determine the one the target is compatible with.
func (p *Provider) detectModelAndEncoding(ctx context.Context, tg *target.Target) (Model, string, error) {
	for _, model := range p.config.Models {
		for _, encoding := range []string{"json_ietf", "json"} {
			subscribeRequestOptions := model.gnmiOptions(api.SubscriptionListModeONCE(), api.Encoding(encoding))
			subscribeReq, err := api.NewSubscribeRequest(subscribeRequestOptions...)
			if err != nil {
				panic(fmt.Errorf("NewSubscribeRequest() error: %w", err))
			}
			_, err = tg.SubscribeOnce(ctx, subscribeReq)
			if err != nil && ctx.Err() != nil {
				return Model{}, "", err
			} else if err != nil {
				// Next encoding or model
				continue
			}
			return model, encoding, nil
		}
	}
	return Model{}, "", fmt.Errorf("no compatible model found")
}
