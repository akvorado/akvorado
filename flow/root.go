// Package flow handle incoming flows (currently Netflow v9).
package flow

import (
	_ "embed" // for flow.proto
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	reuseport "github.com/libp2p/go-reuseport"
	"golang.org/x/time/rate"
	"gopkg.in/tomb.v2"

	"akvorado/daemon"
	"akvorado/flow/decoder"
	"akvorado/http"
	"akvorado/reporter"
)

// Component represents the flow component.
type Component struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	// Metrics
	metrics metrics

	// Channel for sending flows.
	outgoingFlows chan *Message

	// Local address used by the Netflow server. Only valid after Start().
	Address net.Addr
}

// Dependencies are the dependencies of the flow component.
type Dependencies struct {
	Daemon daemon.Component
	HTTP   *http.Component
}

// New creates a new flow component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:             r,
		d:             &dependencies,
		config:        configuration,
		outgoingFlows: make(chan *Message, configuration.QueueSize),
	}
	c.d.Daemon.Track(&c.t, "flow")
	c.initHTTP()
	c.initMetrics()
	return &c, nil
}

// Flows returns a channel to receive flows.
func (c *Component) Flows() <-chan *Message {
	return c.outgoingFlows
}

// Start starts the flow component.
func (c *Component) Start() error {
	decoder := decoder.New("netflow", c.r)

	c.r.Info().Str("listen", c.config.Listen).Msg("starting flow server")
	for i := 0; i < c.config.Workers; i++ {
		if err := c.spawnWorker(i, decoder); err != nil {
			return fmt.Errorf("unable to spawn worker %d: %w", i, err)
		}
	}

	return nil
}

func (c *Component) spawnWorker(workerID int, decoder decoder.Decoder) error {
	// Listen to UDP port
	var listenAddr net.Addr
	if c.Address != nil {
		// We already are listening on one address, let's
		// listen to the same (useful when using :0).
		listenAddr = c.Address
	} else {
		var err error
		listenAddr, err = reuseport.ResolveAddr("udp", c.config.Listen)
		if err != nil {
			return fmt.Errorf("unable to resolve %v: %w", c.config.Listen, err)
		}
	}
	pconn, err := reuseport.ListenPacket("udp", listenAddr.String())
	if err != nil {
		return fmt.Errorf("unable to listen to %v: %w", listenAddr, err)
	}
	udpConn := pconn.(*net.UDPConn)
	c.Address = udpConn.LocalAddr()

	// Go routine for worker
	c.t.Go(func() error {
		errLimiter := rate.NewLimiter(rate.Every(time.Minute), 1)
		workerIDStr := strconv.Itoa(workerID)
		payload := make([]byte, 9000)
		for {
			// Read one packet
			startIdle := time.Now()
			size, source, err := udpConn.ReadFromUDP(payload)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return nil
				}
				if errLimiter.Allow() {
					c.r.Err(err).Int("worker", workerID).Msg("unable to receive UDP packet")
				}
				c.metrics.trafficErrors.WithLabelValues("netflow").Inc()
				continue
			}
			startBusy := time.Now()

			c.metrics.trafficBytes.WithLabelValues(source.IP.String(), "netflow").
				Add(float64(size))
			c.metrics.trafficPackets.WithLabelValues(source.IP.String(), "netflow").
				Inc()
			c.metrics.trafficPacketSizeSum.WithLabelValues(source.IP.String(), "netflow").
				Observe(float64(size))

			c.decodeWith(decoder, payload[:size], source.IP)

			idleTime := float64(startBusy.Sub(startIdle).Nanoseconds()) / 1000 / 1000 / 1000
			busyTime := float64(time.Since(startBusy).Nanoseconds()) / 1000 / 1000 / 1000
			c.metrics.trafficLoopTime.WithLabelValues(workerIDStr, "idle").Observe(idleTime)
			c.metrics.trafficLoopTime.WithLabelValues(workerIDStr, "busy").Observe(busyTime)
		}
	})

	// Watch for termination and close on dying
	c.t.Go(func() error {
		<-c.t.Dying()
		c.r.Debug().Int("worker", workerID).Msg("stopping flow worker")
		udpConn.Close()
		return nil
	})
	return nil
}

// sendFlow transmits received flows to the next component
func (c *Component) sendFlow(fmsg *Message) {
	select {
	case <-c.t.Dying():
		return
	case c.outgoingFlows <- fmsg:
	default:
		// Queue full
		c.metrics.outgoingQueueFullTotal.Inc()
		select {
		case <-c.t.Dying():
			return
		case c.outgoingFlows <- fmsg:
		}
	}
}

// Stop stops the flow component
func (c *Component) Stop() error {
	defer func() {
		close(c.outgoingFlows)
		c.r.Info().Msg("flow component stopped")
	}()
	c.r.Info().Msg("stopping flow component")
	c.t.Kill(nil)
	return c.t.Wait()
}
