package snmp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/gosnmp/gosnmp"
	"golang.org/x/time/rate"

	"flowexporter/reporter"
)

type poller interface {
	Poll(ctx context.Context, host string, port uint16, community string, ifIndex uint)
}

// realPoller will poll hosts using real SNMP requests.
type realPoller struct {
	r     *reporter.Reporter
	clock clock.Clock

	pendingRequests     map[string]bool
	pendingRequestsLock sync.Mutex
	errLimiter          *rate.Limiter
	put                 func(string, uint, Interface)

	metrics struct {
		pendingRequests reporter.GaugeFunc
		successes       *reporter.CounterVec
		failures        *reporter.CounterVec
		retries         *reporter.CounterVec
		times           *reporter.SummaryVec
	}
}

// newPoller creates a new SNMP poller.
func newPoller(r *reporter.Reporter, clock clock.Clock, put func(string, uint, Interface)) *realPoller {
	p := &realPoller{
		r:               r,
		clock:           clock,
		pendingRequests: make(map[string]bool),
		errLimiter:      rate.NewLimiter(rate.Every(10*time.Second), 3),
		put:             put,
	}
	p.metrics.pendingRequests = r.GaugeFunc(
		reporter.GaugeOpts{
			Name: "poller_pending",
			Help: "Number of pending requests in pollers.",
		}, func() float64 {
			p.pendingRequestsLock.Lock()
			defer p.pendingRequestsLock.Unlock()
			return float64(len(p.pendingRequests))
		})
	p.metrics.successes = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_success",
			Help: "Number of successful requests.",
		}, []string{"host"})
	p.metrics.failures = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_failure",
			Help: "Number of failed requests.",
		}, []string{"host", "error"})
	p.metrics.retries = r.CounterVec(
		reporter.CounterOpts{
			Name: "poller_retry",
			Help: "Number of retried requests.",
		}, []string{"host"})
	p.metrics.times = r.SummaryVec(
		reporter.SummaryOpts{
			Name:       "poller_seconds",
			Help:       "Time to successfully poll for values.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}, []string{"host"})
	return p
}

func (p *realPoller) Poll(ctx context.Context, host string, port uint16, community string, ifIndex uint) {
	// Check if already have a request running
	key := fmt.Sprintf("%s@%d", host, ifIndex)
	p.pendingRequestsLock.Lock()
	_, ok := p.pendingRequests[key]
	if !ok {
		p.pendingRequests[key] = true
	}
	p.pendingRequestsLock.Unlock()
	if ok {
		// Request already in progress, skip it
		return
	}
	defer func() {
		p.pendingRequestsLock.Lock()
		delete(p.pendingRequests, key)
		p.pendingRequestsLock.Unlock()
	}()

	// Instantiate an SNMP state
	g := &gosnmp.GoSNMP{
		Target:                  host,
		Port:                    port,
		Community:               community,
		Version:                 gosnmp.Version2c,
		Context:                 ctx,
		Retries:                 3,
		Timeout:                 time.Second,
		UseUnconnectedUDPSocket: true,
		Logger:                  gosnmp.NewLogger(&goSNMPLogger{p.r}),
		OnRetry: func(*gosnmp.GoSNMP) {
			p.metrics.retries.WithLabelValues(host).Inc()
		},
	}
	if err := g.Connect(); err != nil {
		p.metrics.failures.WithLabelValues(host, "connect").Inc()
		if p.errLimiter.Allow() {
			p.r.Err(err).Str("host", host).Msg("unable to connect")
		}
	}
	start := p.clock.Now()
	ifDescr := fmt.Sprintf("1.3.6.1.2.1.2.2.1.2.%d", ifIndex)
	ifAlias := fmt.Sprintf("1.3.6.1.2.1.31.1.1.1.18.%d", ifIndex)
	result, err := g.Get([]string{ifDescr, ifAlias})
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		p.metrics.failures.WithLabelValues(host, "get").Inc()
		if p.errLimiter.Allow() {
			p.r.Err(err).Str("host", host).Msg("unable to get")
		}
		return
	}

	ok = true
	switch result.Variables[0].Type {
	case gosnmp.OctetString:
		ifDescr = string(result.Variables[0].Value.([]byte))
	case gosnmp.NoSuchInstance, gosnmp.NoSuchObject:
		p.metrics.failures.WithLabelValues(host, "ifdescr_missing").Inc()
		ok = false
	default:
		p.metrics.failures.WithLabelValues(host, "ifdescr_unknown_type").Inc()
		ok = false
	}
	switch result.Variables[1].Type {
	case gosnmp.OctetString:
		ifAlias = string(result.Variables[1].Value.([]byte))
	case gosnmp.NoSuchInstance, gosnmp.NoSuchObject:
		p.metrics.failures.WithLabelValues(host, "ifalias_missing").Inc()
		ok = false
	default:
		p.metrics.failures.WithLabelValues(host, "ifalias_unknown_type").Inc()
		ok = false
	}
	if !ok {
		return
	}
	p.put(host, ifIndex, Interface{
		Name:        ifDescr,
		Description: ifAlias,
	})

	p.metrics.successes.WithLabelValues(host).Inc()
	p.metrics.times.WithLabelValues(host).Observe(p.clock.Now().Sub(start).Seconds())
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
