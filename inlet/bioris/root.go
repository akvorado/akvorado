package bioris

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net/netip"
	"time"

	pb "github.com/bio-routing/bio-rd/cmd/ris/api"
	bnet "github.com/bio-routing/bio-rd/net"
	rpb "github.com/bio-routing/bio-rd/route/api"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/tomb.v2"

	"akvorado/common/daemon"
	"akvorado/common/reporter"
	"akvorado/inlet/bmp"
)

// RISInstanceRuntime represents all connections etc. to a single RIS
type RISInstanceRuntime struct {
	conn   *grpc.ClientConn
	client pb.RoutingInformationServiceClient
	config RISInstance
}

// Component represents the BioRIS component.
type Component struct {
	r             *reporter.Reporter
	d             *Dependencies
	t             tomb.Tomb
	i             map[string]*RISInstanceRuntime
	log           reporter.Logger
	metrics       metrics
	config        Configuration
	router        map[netip.Addr][]*RISInstanceRuntime
	clientMetrics *grpc_prometheus.ClientMetrics
}

// Dependencies define the dependencies of the BioRis component.
type Dependencies struct {
	Daemon daemon.Component
}

// New creates a new BioRIS component.
func New(r *reporter.Reporter, configuration Configuration, dependencies Dependencies) (*Component, error) {
	c := Component{
		r:      r,
		d:      &dependencies,
		i:      make(map[string]*RISInstanceRuntime),
		config: configuration,
		router: make(map[netip.Addr][]*RISInstanceRuntime),
	}
	c.d.Daemon.Track(&c.t, "inlet/bioris")
	c.clientMetrics = grpc_prometheus.NewClientMetrics()
	c.initMetrics()

	return &c, nil
}

// Start starts the bioris component
func (c *Component) Start() error {
	c.r.Info().Msg("starting BioRIS component")
	rand.Seed(time.Now().Unix())
	for _, con := range c.config.RISInstances {
		securityOption := grpc.WithTransportCredentials(insecure.NewCredentials())

		if con.GRPCSecure {
			config := &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
			securityOption = grpc.WithTransportCredentials(credentials.NewTLS(config))
		}
		backoff := backoff.DefaultConfig

		conn, err := grpc.Dial(con.GRPCAddr, securityOption,
			grpc.WithUnaryInterceptor(c.clientMetrics.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(c.clientMetrics.StreamClientInterceptor()),
			grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff}),
		)
		if err != nil {
			c.metrics.risUp.WithLabelValues(con.GRPCAddr).Set(0)
			c.log.Err(err).Msg("err while dialing RIS " + con.GRPCAddr)
			continue
		}
		client := pb.NewRoutingInformationServiceClient(conn)
		if client == nil {
			c.metrics.risUp.WithLabelValues(con.GRPCAddr).Set(0)
			// we only fail softly here, as a single unavailable client is no reason for the complete inlet to crash
			c.log.Error().Msg("err while opening RoutingInformationServiceClient " + con.GRPCAddr)
			continue
		}
		c.metrics.risUp.WithLabelValues(con.GRPCAddr).Set(1)

		r, err := client.GetRouters(context.Background(), &pb.GetRoutersRequest{})
		if err != nil {
			c.metrics.risUp.WithLabelValues(con.GRPCAddr).Set(0)
			// we only fail softly here, as a single unavailable client is no reason for the complete inlet to crash
			c.log.Err(err).Msg("err while getting routers from " + con.GRPCAddr)
			continue
		}

		c.i[con.GRPCAddr] = &RISInstanceRuntime{
			config: con,
			client: client,
			conn:   conn,
		}

		for _, router := range r.GetRouters() {
			routerAddress, e := netip.ParseAddr(router.Address)

			if e != nil {
				c.log.Err(e).Msg("err while parsing router address " + router.Address)
				continue
			}
			// akvorado handles everything as IPv6-mapped addr. Therefore, we also convert our router id to ipv6 mapped
			routerAddress = netip.AddrFrom16(routerAddress.As16())

			c.router[routerAddress] = append(c.router[routerAddress], c.i[con.GRPCAddr])
			c.metrics.knownRouters.WithLabelValues(con.GRPCAddr).Inc()
			// we need to initialize all the counters here
			c.metrics.lpmRequestContextCanceled.WithLabelValues(con.GRPCAddr, router.Address)
			c.metrics.lpmRequestErrors.WithLabelValues(con.GRPCAddr, router.Address)
			c.metrics.lpmRequestSuccess.WithLabelValues(con.GRPCAddr, router.Address)
			c.metrics.lpmRequests.WithLabelValues(con.GRPCAddr, router.Address)
			c.metrics.routerChosenAgentIDMatch.WithLabelValues(con.GRPCAddr, router.Address)
			c.metrics.routerChosenRandom.WithLabelValues(con.GRPCAddr, router.Address)
		}
	}
	return nil
}

// choose router selects the the router id best suited for the given agent ip. It returns router id and ris instance
func (c *Component) chooseRouter(agent netip.Addr) (netip.Addr, *RISInstanceRuntime, error) {
	var chosenRis *RISInstanceRuntime
	chosenRouterID := netip.IPv4Unspecified()
	// all routers that could be of interest for us
	var routers []netip.Addr
	exactMatch := false
	// First try: Try to found router which exactly matches the agent id
	for r := range c.router {
		if r == agent {
			routers = append(routers, r)
			exactMatch = true
		}
	}
	// second try: Choose a random router, if no exact match was found yet
	if !exactMatch {
		for r := range c.router {
			routers = append(routers, r)
		}
	}

	if len(routers) < 1 {
		return chosenRouterID, nil, errors.New("no applicable router found for bio flow lookup")
	}
	// now choose one of the routers
	chosenRouterID = routers[rand.Intn(len(routers))]

	// randomly select a ris providing the router id
	chosenRis = c.router[chosenRouterID][rand.Intn(len(c.router[chosenRouterID]))]

	// update metrics with the chosen router/ris combination
	if exactMatch {
		c.metrics.routerChosenAgentIDMatch.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.String()).Inc()
	} else {
		c.metrics.routerChosenRandom.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.String()).Inc()
	}

	if chosenRis == nil || chosenRouterID.IsUnspecified() {
		return chosenRouterID, nil, errors.New("no ris available for bio flow lookup")
	}
	return chosenRouterID, chosenRis, nil
}

func (c *Component) lpmResponseToLookupResult(lpm *pb.LPMResponse) (bmp.LookupResult, error) {
	var res bmp.LookupResult

	res.ASN = 0

	var r *rpb.Route
	largestPfxLen := -1
	if lpm == nil {
		return res, fmt.Errorf("lpm: result empty")
	}

	// first: Find longest matching prefix under all applicable routes
	for _, tr := range lpm.Routes {
		if int(tr.Pfx.Length) > largestPfxLen {
			// we have found a better prefix, set that as the currently used one
			r = tr
			largestPfxLen = int(tr.Pfx.Length)
		}
	}

	if r == nil {
		return res, fmt.Errorf("lpm: no route returned")
	}

	// assume the first path is the preferred path, we are interested only in that path
	if len(r.Paths) < 1 {
		return res, fmt.Errorf("lpm: no path found")
	}
	p := r.Paths[0]
	if p == nil {
		return res, fmt.Errorf("lpm: path is nil")
	}

	if p.BgpPath == nil {
		return res, fmt.Errorf("lpm: path has no bgp path")
	}

	res.Communities = append(res.Communities, p.BgpPath.Communities...)
	for _, c := range p.BgpPath.LargeCommunities {
		res.LargeCommunities = append(res.LargeCommunities, *bgp.NewLargeCommunity(c.GetGlobalAdministrator(), c.GetDataPart1(), c.GetDataPart2()))
	}

	for _, asP := range p.BgpPath.AsPath {
		for _, as := range asP.Asns {
			res.ASPath = append(res.ASPath, as)
			res.ASN = as
		}
	}

	res.NetMask = uint8(r.Pfx.GetLength())
	return res, nil
}

// Lookup does an lookup on one of the specified RIS Instances, and returns the well known bmp lookup result. NextHopIP is ignored, but maintained for compatibility to the internal bmp
func (c *Component) Lookup(addrIP netip.Addr, agentIP netip.Addr, _ netip.Addr) (bmp.LookupResult, error) {

	lpmRes, lpmErr := c.LPM(addrIP, agentIP)

	if lpmErr != nil {
		return bmp.LookupResult{}, lpmErr
	}
	r, err := c.lpmResponseToLookupResult(lpmRes)
	if err != nil {
		c.r.Logger.Warn().Msgf("loopup %s error: %v", addrIP.String(), err)
	}
	return r, err
}

// LPM does an LPM GRPC call to an BioRis instance
func (c *Component) LPM(ip netip.Addr, agent netip.Addr) (*pb.LPMResponse, error) {
	// first step: choose router id and ris
	chosenRouterID, chosenRis, err := c.chooseRouter(agent)
	if err != nil {
		return nil, err
	}

	ipAddr, err := bnet.IPFromString(ip.String())
	if err != nil {
		return nil, err
	}

	pfxLen := uint8(32)
	if !ipAddr.IsIPv4() {
		pfxLen = 128
	}
	pfx := bnet.NewPfx(ipAddr, pfxLen)

	c.metrics.lpmRequests.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.String()).Inc()

	clientDeadline := time.Now().Add(time.Duration(200) * time.Millisecond)
	ctx, _ := context.WithDeadline(context.Background(), clientDeadline)

	var res *pb.LPMResponse
	// attention: the ris does not understand ipv6-mapped router ids, so we need to unmap them
	res, err = chosenRis.client.LPM(ctx, &pb.LPMRequest{
		Router: chosenRouterID.Unmap().String(),
		VrfId:  chosenRis.config.VRFId,
		Vrf:    chosenRis.config.VRF,
		Pfx:    pfx.ToProto(),
	})
	if errors.Is(ctx.Err(), context.Canceled) {
		c.metrics.lpmRequestContextCanceled.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.String()).Inc()
		return nil, errors.New("lpm lookup canceled")
	}
	if err != nil {
		c.metrics.lpmRequestErrors.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.String()).Inc()
		return nil, err
	}

	c.metrics.lpmRequestSuccess.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.String()).Inc()
	return res, nil
}

// Stop closes connection to ris
func (c *Component) Stop() error {
	for _, v := range c.i {
		v.conn.Close()
	}
	return nil
}
