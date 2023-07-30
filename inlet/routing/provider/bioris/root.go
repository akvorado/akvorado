package bioris

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net/netip"
	"sync"
	"time"

	pb "github.com/bio-routing/bio-rd/cmd/ris/api"
	bnet "github.com/bio-routing/bio-rd/net"
	rpb "github.com/bio-routing/bio-rd/route/api"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/tomb.v2"

	"akvorado/common/reporter"
	"akvorado/inlet/routing/provider"
	"akvorado/inlet/routing/provider/bmp"
)

// RISInstanceRuntime represents all connections to a single RIS
type RISInstanceRuntime struct {
	conn   *grpc.ClientConn
	client pb.RoutingInformationServiceClient
	config RISInstance
}

// Provider represents the BioRIS routing provider.
type Provider struct {
	r      *reporter.Reporter
	d      *Dependencies
	t      tomb.Tomb
	config Configuration

	metrics       metrics
	clientMetrics *grpc_prometheus.ClientMetrics
	instances     map[string]*RISInstanceRuntime
	routers       map[netip.Addr][]*RISInstanceRuntime
	mu            sync.RWMutex
}

// Dependencies define the dependencies of the BioRIS Provider.
type Dependencies = provider.Dependencies

// New creates a new BioRIS provider.
func (configuration Configuration) New(r *reporter.Reporter, dependencies Dependencies) (provider.Provider, error) {
	p := Provider{
		r:         r,
		d:         &dependencies,
		config:    configuration,
		instances: make(map[string]*RISInstanceRuntime),
		routers:   make(map[netip.Addr][]*RISInstanceRuntime),
	}
	p.clientMetrics = grpc_prometheus.NewClientMetrics()
	p.initMetrics()

	return &p, nil
}

// Start starts the bioris provider.
func (p *Provider) Start() error {
	p.r.Info().Msg("starting BioRIS provider")

	// Connect to RIS backend (done in background)
	for _, config := range p.config.RISInstances {
		instance, err := p.Dial(config)
		if err != nil {
			return fmt.Errorf("error while dialing %s: %w", config.GRPCAddr, err)
		}
		p.instances[config.GRPCAddr] = instance
	}

	refresh := func(ctx context.Context) {
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(p.config.RefreshTimeout))
		defer cancel()
		p.Refresh(ctx)
	}
	refresh(context.Background())
	p.d.Daemon.Track(&p.t, "inlet/bmp")
	p.t.Go(func() error {
		ticker := time.NewTicker(p.config.Refresh)
		defer ticker.Stop()
		for {
			select {
			case <-p.t.Dying():
				return nil
			case <-ticker.C:
				refresh(p.t.Context(context.Background()))
			}
		}
	})

	return nil
}

// Dial dials a RIS instance.
func (p *Provider) Dial(config RISInstance) (*RISInstanceRuntime, error) {
	securityOption := grpc.WithTransportCredentials(insecure.NewCredentials())

	if config.GRPCSecure {
		config := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		securityOption = grpc.WithTransportCredentials(credentials.NewTLS(config))
	}
	backoff := backoff.DefaultConfig
	conn, err := grpc.Dial(config.GRPCAddr, securityOption,
		grpc.WithUnaryInterceptor(p.clientMetrics.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(p.clientMetrics.StreamClientInterceptor()),
		grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff}),
	)
	if err != nil {
		return nil, fmt.Errorf("error while dialing RIS %s: %w", config.GRPCAddr, err)
	}
	client := pb.NewRoutingInformationServiceClient(conn)
	if client == nil {
		conn.Close()
		return nil, fmt.Errorf("error while opening RIS client %s", config.GRPCAddr)
	}
	p.t.Go(func() error {
		var state connectivity.State = -1
		for {
			if !conn.WaitForStateChange(p.t.Context(context.Background()), state) {
				return nil
			}
			state = conn.GetState()
			p.metrics.risUp.WithLabelValues(config.GRPCAddr).Set(func() float64 {
				if state == connectivity.Ready {
					return 1
				}
				return 0
			}())
			state = conn.GetState()
		}
	})

	return &RISInstanceRuntime{
		config: config,
		client: client,
		conn:   conn,
	}, nil
}

// Refresh retrieves the list of routers
func (p *Provider) Refresh(ctx context.Context) {
	routers := make(map[netip.Addr][]*RISInstanceRuntime)
	for _, config := range p.config.RISInstances {
		instance := p.instances[config.GRPCAddr]
		r, err := instance.client.GetRouters(ctx, &pb.GetRoutersRequest{})
		if err != nil {
			p.r.Err(err).Msgf("error while getting routers from %s", config.GRPCAddr)
			continue
		}
		p.metrics.knownRouters.WithLabelValues(config.GRPCAddr).Set(0)
		for _, router := range r.GetRouters() {
			routerAddress, err := netip.ParseAddr(router.Address)
			if err != nil {
				p.r.Err(err).Msgf("error while parsing router address %s", router.Address)
				continue
			}
			routerAddress = netip.AddrFrom16(routerAddress.As16())
			routers[routerAddress] = append(routers[routerAddress], p.instances[config.GRPCAddr])

			p.metrics.knownRouters.WithLabelValues(config.GRPCAddr).Inc()
			p.metrics.lpmRequestTimeouts.WithLabelValues(config.GRPCAddr, router.Address)
			p.metrics.lpmRequestErrors.WithLabelValues(config.GRPCAddr, router.Address)
			p.metrics.lpmRequestSuccess.WithLabelValues(config.GRPCAddr, router.Address)
			p.metrics.lpmRequests.WithLabelValues(config.GRPCAddr, router.Address)
			p.metrics.routerChosenAgentIDMatch.WithLabelValues(config.GRPCAddr, router.Address)
			p.metrics.routerChosenFallback.WithLabelValues(config.GRPCAddr, router.Address)
		}
	}

	p.mu.Lock()
	p.routers = routers
	p.mu.Unlock()
}

// Lookup does an lookup on one of the specified RIS Instances and returns the
// well known bmp lookup result. NextHopIP is ignored, but maintained for
// compatibility to the internal bmp
func (p *Provider) Lookup(ctx context.Context, ip netip.Addr, _ netip.Addr, agent netip.Addr) (provider.LookupResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	lpmRes, lpmErr := p.lookupLPM(ctx, ip, agent)

	if lpmErr != nil {
		return bmp.LookupResult{}, lpmErr
	}
	r, err := p.lpmResponseToLookupResult(lpmRes)
	if err != nil {
		return bmp.LookupResult{}, err
	}
	return r, nil
}

// chooseRouter selects the router ID best suited for the given agent ip. It
// returns router ID and RIS instance.
func (p *Provider) chooseRouter(agent netip.Addr) (netip.Addr, *RISInstanceRuntime, error) {
	var chosenRis *RISInstanceRuntime
	chosenRouterID := netip.IPv4Unspecified()
	exactMatch := false
	// We try all routers
	for r := range p.routers {
		chosenRouterID = r
		// If we find an exact match of router id and agent ip, we are done
		if r == agent {
			exactMatch = true
			break
		}
		// If not, we are implicitly using the last router id we found
	}

	// Verify that an actual router was found
	if chosenRouterID.IsUnspecified() {
		return chosenRouterID, nil, errors.New("no applicable router found for flow lookup")
	}

	// Randomly select a ris providing the router ID we selected earlier.
	// In the future, we might also want to exclude currently unavailable ris instances
	chosenRis = p.routers[chosenRouterID][rand.Intn(len(p.routers[chosenRouterID]))]

	if chosenRis == nil || chosenRouterID.IsUnspecified() {
		return chosenRouterID, nil, errors.New("no instance available for flow lookup")
	}

	// Update metrics with the chosen router/ris combination
	if exactMatch {
		p.metrics.routerChosenAgentIDMatch.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.Unmap().String()).Inc()
	} else {
		p.metrics.routerChosenFallback.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.Unmap().String()).Inc()
	}

	return chosenRouterID, chosenRis, nil
}

func (p *Provider) lpmResponseToLookupResult(lpm *pb.LPMResponse) (bmp.LookupResult, error) {
	var res bmp.LookupResult

	res.ASN = 0

	var r *rpb.Route
	largestPfxLen := -1
	if lpm == nil {
		return res, fmt.Errorf("lpm: result empty")
	}

	// First: find longest matching prefix under all applicable routes
	for _, tr := range lpm.Routes {
		if int(tr.Pfx.Length) > largestPfxLen {
			// We have found a better prefix, set that as the currently used one
			r = tr
			largestPfxLen = int(tr.Pfx.Length)
		}
	}

	if r == nil {
		return res, fmt.Errorf("lpm: no route returned")
	}

	// Assume the first path is the preferred path, we are interested only in that path
	if len(r.Paths) < 1 {
		return res, fmt.Errorf("lpm: no path found")
	}
	pfx := r.Paths[0]
	if pfx == nil {
		return res, fmt.Errorf("lpm: path is nil")
	}

	if pfx.BgpPath == nil {
		return res, fmt.Errorf("lpm: path has no BGP path")
	}

	res.Communities = append(res.Communities, pfx.BgpPath.Communities...)
	for _, c := range pfx.BgpPath.LargeCommunities {
		res.LargeCommunities = append(res.LargeCommunities,
			*bgp.NewLargeCommunity(c.GetGlobalAdministrator(), c.GetDataPart1(), c.GetDataPart2()))
	}

	for _, asP := range pfx.BgpPath.AsPath {
		for _, as := range asP.Asns {
			res.ASPath = append(res.ASPath, as)
			res.ASN = as
		}
	}

	res.NetMask = uint8(r.Pfx.GetLength())
	return res, nil
}

// lookupLPM does an lookupLPM GRPC call to a BioRis instance
func (p *Provider) lookupLPM(ctx context.Context, ip netip.Addr, agent netip.Addr) (*pb.LPMResponse, error) {
	// Choose router id and ris
	chosenRouterID, chosenRis, err := p.chooseRouter(agent)
	if err != nil {
		return nil, err
	}

	ipAddr, err := bnet.IPFromBytes(ip.Unmap().AsSlice())
	if err != nil {
		return nil, err
	}

	pfxLen := uint8(32)
	if !ipAddr.IsIPv4() {
		pfxLen = 128
	}
	pfx := bnet.NewPfx(ipAddr, pfxLen)

	p.metrics.lpmRequests.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.Unmap().String()).Inc()

	clientDeadline := time.Now().Add(p.config.Timeout)
	ctx, cancel := context.WithDeadline(ctx, clientDeadline)
	defer cancel()

	var res *pb.LPMResponse
	res, err = chosenRis.client.LPM(ctx, &pb.LPMRequest{
		Router: chosenRouterID.Unmap().String(),
		VrfId:  chosenRis.config.VRFId,
		Vrf:    chosenRis.config.VRF,
		Pfx:    pfx.ToProto(),
	})
	if errors.Is(ctx.Err(), context.Canceled) {
		p.metrics.lpmRequestTimeouts.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.Unmap().String()).Inc()
		return nil, errors.New("lpm lookup timeout")
	}
	if err != nil {
		p.metrics.lpmRequestErrors.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.Unmap().String()).Inc()
		return nil, fmt.Errorf("lpm lookup failed: %w", err)
	}

	p.metrics.lpmRequestSuccess.WithLabelValues(chosenRis.config.GRPCAddr, chosenRouterID.Unmap().String()).Inc()
	return res, nil
}

// Stop closes connection to ris
func (p *Provider) Stop() error {
	defer func() {
		for _, v := range p.instances {
			if v.conn != nil {
				v.conn.Close()
			}
		}
		p.r.Info().Msg("BioRIS provider stopped")
	}()
	p.r.Info().Msg("stopping BioRIS provider")
	p.t.Kill(nil)
	return p.t.Wait()
}
