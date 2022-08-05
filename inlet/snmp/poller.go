// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package snmp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/gosnmp/gosnmp"

	"akvorado/common/reporter"
)

type poller interface {
	Poll(ctx context.Context, exporterIP string, port uint16, community string, ifIndexes []uint) error
}

// realPoller will poll exporters using real SNMP requests.
type realPoller struct {
	r      *reporter.Reporter
	config pollerConfig
	clock  clock.Clock

	pendingRequests     map[string]struct{}
	pendingRequestsLock sync.Mutex
	errLogger           reporter.Logger
	put                 func(exporterIP, exporterName string, ifIndex uint, iface Interface)

	metrics struct {
		pendingRequests reporter.GaugeFunc
		successes       *reporter.CounterVec
		failures        *reporter.CounterVec
		retries         *reporter.CounterVec
		times           *reporter.SummaryVec
	}
}

type pollerConfig struct {
	Retries int
	Timeout time.Duration
}

// newPoller creates a new SNMP poller.
func newPoller(r *reporter.Reporter, config pollerConfig, clock clock.Clock, put func(string, string, uint, Interface)) *realPoller {
	p := &realPoller{
		r:               r,
		config:          config,
		clock:           clock,
		pendingRequests: make(map[string]struct{}),
		errLogger:       r.Sample(reporter.BurstSampler(10*time.Second, 3)),
		put:             put,
	}
	p.metrics.pendingRequests = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "poller_pending_requests",
			Help: "Number of pending requests in pollers.",
		}, func() float64 {
			p.pendingRequestsLock.Lock()
			defer p.pendingRequestsLock.Unlock()
			return float64(len(p.pendingRequests))
		})
	p.metrics.successes = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_success_requests",
			Help: "Number of successful requests.",
		}, []string{"exporter"})
	p.metrics.failures = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_failure_requests",
			Help: "Number of failed requests.",
		}, []string{"exporter", "error"})
	p.metrics.retries = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_retry_requests",
			Help: "Number of retried requests.",
		}, []string{"exporter"})
	p.metrics.times = r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "poller_seconds",
			Help:       "Time to successfully poll for values.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"exporter"})
	return p
}

func (p *realPoller) Poll(ctx context.Context, exporter string, port uint16, community string, ifIndexes []uint) error {
	// Check if already have a request running
	filteredIfIndexes := make([]uint, 0, len(ifIndexes))
	keys := make([]string, 0, len(ifIndexes))
	p.pendingRequestsLock.Lock()
	for _, ifIndex := range ifIndexes {
		key := fmt.Sprintf("%s@%d", exporter, ifIndex)
		_, ok := p.pendingRequests[key]
		if !ok {
			p.pendingRequests[key] = struct{}{}
			filteredIfIndexes = append(filteredIfIndexes, ifIndex)
			keys = append(keys, key)
		}
	}
	p.pendingRequestsLock.Unlock()
	if len(filteredIfIndexes) == 0 {
		return nil
	}
	ifIndexes = filteredIfIndexes
	defer func() {
		p.pendingRequestsLock.Lock()
		for _, key := range keys {
			delete(p.pendingRequests, key)
		}
		p.pendingRequestsLock.Unlock()
	}()

	// Instantiate an SNMP state
	g := &gosnmp.GoSNMP{
		Target:                  exporter,
		Port:                    port,
		Community:               community,
		Version:                 gosnmp.Version2c,
		Context:                 ctx,
		Retries:                 p.config.Retries,
		Timeout:                 p.config.Timeout,
		UseUnconnectedUDPSocket: true,
		Logger:                  gosnmp.NewLogger(&goSNMPLogger{p.r}),
		OnRetry: func(*gosnmp.GoSNMP) {
			p.metrics.retries.WithLabelValues(exporter).Inc()
		},
	}
	if err := g.Connect(); err != nil {
		p.metrics.failures.WithLabelValues(exporter, "connect").Inc()
		p.errLogger.Err(err).Str("exporter", exporter).Msg("unable to connect")
	}
	start := p.clock.Now()
	requests := []string{"1.3.6.1.2.1.1.5.0"}
	for _, ifIndex := range ifIndexes {
		moreRequests := []string{
			fmt.Sprintf("1.3.6.1.2.1.2.2.1.2.%d", ifIndex),     // ifDescr
			fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", ifIndex), // ifAlias
			fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.15.%d", ifIndex), // ifSpeed
		}
		requests = append(requests, moreRequests...)
	}
	result, err := g.Get(requests)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err != nil {
		p.metrics.failures.WithLabelValues(exporter, "get").Inc()
		p.errLogger.Err(err).
			Str("exporter", exporter).
			Msgf("unable to GET (%d OIDs)", len(requests))
		return err
	}
	if result.Error != gosnmp.NoError && result.ErrorIndex == 0 {
		// There is some error affecting the whole request
		p.metrics.failures.WithLabelValues(exporter, "get").Inc()
		p.errLogger.Error().
			Str("exporter", exporter).
			Stringer("code", result.Error).
			Msgf("unable to GET (%d OIDs)", len(requests))
		return fmt.Errorf("SNMP error %s(%d)", result.Error, result.Error)
	}

	processStr := func(idx int, what string, target *string, mandatory bool) bool {
		switch result.Variables[idx].Type {
		case gosnmp.OctetString:
			*target = string(result.Variables[idx].Value.([]byte))
		case gosnmp.NoSuchInstance, gosnmp.NoSuchObject:
			if mandatory {
				p.metrics.failures.WithLabelValues(exporter, fmt.Sprintf("%s missing", what)).Inc()
				return false
			}
		default:
			p.metrics.failures.WithLabelValues(exporter, fmt.Sprintf("%s unknown type", what)).Inc()
			return false
		}
		return true
	}
	processUint := func(idx int, what string, target *uint, mandatory bool) bool {
		switch result.Variables[idx].Type {
		case gosnmp.Gauge32:
			*target = result.Variables[idx].Value.(uint)
		case gosnmp.NoSuchInstance, gosnmp.NoSuchObject:
			if mandatory {
				p.metrics.failures.WithLabelValues(exporter, fmt.Sprintf("%s missing", what)).Inc()
				return false
			}
		default:
			p.metrics.failures.WithLabelValues(exporter, fmt.Sprintf("%s unknown type", what)).Inc()
			return false
		}
		return true
	}
	var (
		sysNameVal string
		ifDescrVal = "unknown"
		ifAliasVal string
		ifSpeedVal uint
	)
	if !processStr(0, "sysname", &sysNameVal, true) {
		return errors.New("unable to get sysName")
	}
	for idx := 1; idx < len(requests)-2; idx += 3 {
		ifIndex := ifIndexes[(idx-1)/3]
		ok := true
		if !processStr(idx, "ifdescr", &ifDescrVal, ifIndex > 0) {
			ok = false
		}
		if !processStr(idx+1, "ifalias", &ifAliasVal, ifIndex > 0) {
			ok = false
		}
		if !processUint(idx+2, "ifspeed", &ifSpeedVal, ifIndex > 0) {
			ok = false
		}
		if !ok {
			continue
		}
		p.put(exporter, sysNameVal, ifIndex, Interface{
			Name:        ifDescrVal,
			Description: ifAliasVal,
			Speed:       ifSpeedVal,
		})
		p.metrics.successes.WithLabelValues(exporter).Inc()
	}

	p.metrics.times.WithLabelValues(exporter).Observe(p.clock.Now().Sub(start).Seconds())
	return nil
}

type goSNMPLogger struct {
	r *reporter.Reporter
}

func (l *goSNMPLogger) Print(v ...interface{}) {
	if e := l.r.Debug(); e.Enabled() {
		e.Msg(fmt.Sprint(v...))
	}
}
func (l *goSNMPLogger) Printf(format string, v ...interface{}) {
	if e := l.r.Debug(); e.Enabled() {
		e.Msg(fmt.Sprintf(format, v...))
	}
}
