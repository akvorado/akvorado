// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bioris

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"path"
	"testing"
	"time"

	pb "github.com/bio-routing/bio-rd/cmd/ris/api"
	"github.com/bio-routing/bio-rd/cmd/ris/risserver"
	bnet "github.com/bio-routing/bio-rd/net"
	"github.com/bio-routing/bio-rd/protocols/bgp/server"
	rpb "github.com/bio-routing/bio-rd/route/api"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/routing/provider"
)

func TestChooseRouter(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	p, err := config.New(r, provider.Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, p)
	c := p.(*Provider)

	// First test: we have no routers/ris instances and fail with an error
	t.Run("no router", func(t *testing.T) {
		expected := "no router"
		_, _, err := c.chooseRouter(netip.MustParseAddr("10.0.0.0"))
		if diff := helpers.Diff(err.Error(), expected); diff != "" {
			t.Errorf("Error (-got, +want):\n%s", diff)
		}
	})

	// Create some RisInstanceRuntime objects
	ris1 := &RISInstanceRuntime{config: RISInstance{GRPCAddr: "ris1"}}
	ris2 := &RISInstanceRuntime{config: RISInstance{GRPCAddr: "ris2"}}
	ris3 := &RISInstanceRuntime{config: RISInstance{GRPCAddr: "ris3"}}

	// Add them to the component
	c.instances["ris1"] = ris1
	c.instances["ris2"] = ris2
	c.instances["ris3"] = ris3

	// Create a few routers
	r1 := netip.MustParseAddr("10.0.0.1")
	r2 := netip.MustParseAddr("10.0.0.2")
	r3 := netip.MustParseAddr("10.0.0.3")
	r4 := netip.MustParseAddr("10.0.0.4")
	r5 := netip.MustParseAddr("10.0.0.5")

	// Add routers to the ris components
	// r1 is on ris1 and ris3
	c.routers[r1] = []*RISInstanceRuntime{ris1, ris3}
	// r2 is on ris2
	c.routers[r2] = []*RISInstanceRuntime{ris2}
	// r3 is on ris1 and ris3
	c.routers[r3] = []*RISInstanceRuntime{ris1, ris3}
	// r4 is on ris2
	c.routers[r4] = []*RISInstanceRuntime{ris2}
	// r5 is on ris1
	c.routers[r5] = []*RISInstanceRuntime{ris1}

	// Test exact match for r1
	t.Run("exact match r1", func(t *testing.T) {
		// expectedRis is a list of expected ris instances (ris1 and ris3)
		expectedRis := []*RISInstanceRuntime{ris1, ris3}
		expectedRouter := r1

		router, ris, err := c.chooseRouter(netip.MustParseAddr("10.0.0.1"))
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if diff := helpers.Diff(router, expectedRouter); diff != "" {
			t.Errorf("Router (-got, +want):\n%s", diff)
		}
		// Check if ris is in the list of expected ris instances, if not fail
		if !slices.Contains(expectedRis, ris) {
			t.Errorf("Unexpected ris instance: %s", ris.config.GRPCAddr)
		}
	})
	t.Run("exact match r2", func(t *testing.T) {
		expectedRis := []*RISInstanceRuntime{ris2}
		expectedRouter := r2

		router, ris, err := c.chooseRouter(netip.MustParseAddr("10.0.0.2"))
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if diff := helpers.Diff(router, expectedRouter); diff != "" {
			t.Errorf("Router (-got, +want):\n%s", diff)
		}
		if !slices.Contains(expectedRis, ris) {
			t.Errorf("Unexpected ris instance: %s", ris.config.GRPCAddr)
		}
	})
	t.Run("random match", func(t *testing.T) {
		expectedRis := []*RISInstanceRuntime{ris1, ris2, ris3}
		expectedRouter := []netip.Addr{r1, r2, r3, r4, r5}

		router, ris, err := c.chooseRouter(netip.MustParseAddr("9.9.9.9"))
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if !slices.Contains(expectedRouter, router) {
			t.Errorf("Unexpected router: %s", router)
		}
		if !slices.Contains(expectedRis, ris) {
			t.Errorf("Unexpected ris instance: %s", ris.config.GRPCAddr)
		}
	})
}

func TestLPMResponseToLookupResult(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	p, err := config.New(r, provider.Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, p)
	c := p.(*Provider)

	// create some bnet prefixes
	p1Ip, _ := bnet.IPFromString("::")
	p1 := bnet.NewPfx(p1Ip, 0)
	p2Ip, _ := bnet.IPFromString("2001:db8::")
	p2 := bnet.NewPfx(p2Ip, 32)

	cases := []struct {
		name     string
		lpm      *pb.LPMResponse
		expected provider.LookupResult
		err      string
	}{
		{
			name:     "LPM without route",
			lpm:      &pb.LPMResponse{},
			expected: provider.LookupResult{},
			err:      "no route found",
		},
		{
			name:     "LPM is nil",
			lpm:      nil,
			expected: provider.LookupResult{},
			err:      "result empty",
		},
		{
			name: "LPM without path",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{},
			},
			expected: provider.LookupResult{},
			err:      "no route found",
		},
		{
			name: "LPM with empty path",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{
					{
						Pfx:   p1.ToProto(),
						Paths: []*rpb.Path{},
					},
				},
			},
			expected: provider.LookupResult{},
			err:      "no path found",
		},
		{
			name: "LPM with nil path",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{
					{
						Pfx: p1.ToProto(),
						Paths: []*rpb.Path{
							{},
						},
					},
				},
			},
			expected: provider.LookupResult{},
			err:      "no path found",
		},
		{
			name: "LPM with default route and more specific, content in BGP Path",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{
					{
						Pfx: p1.ToProto(),
						Paths: []*rpb.Path{
							{},
						},
					},
					{
						Pfx: p2.ToProto(),
						Paths: []*rpb.Path{
							{
								BgpPath: &rpb.BGPPath{
									Communities: []uint32{123},
									LargeCommunities: []*rpb.LargeCommunity{
										{
											DataPart1: 123,
											DataPart2: 456,
										},
									},

									AsPath: []*rpb.ASPathSegment{
										{
											Asns: []uint32{123, 456},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: provider.LookupResult{
				ASN:              456,
				ASPath:           []uint32{123, 456},
				Communities:      []uint32{123},
				LargeCommunities: []bgp.LargeCommunity{{LocalData1: 123, LocalData2: 456}},
				NetMask:          32,
			},
			err: "",
		},
		{
			name: "LPM with default route and more specific, no content in BGP Path",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{
					{
						Pfx: p1.ToProto(),
						Paths: []*rpb.Path{
							{},
						},
					},
					{
						Pfx: p2.ToProto(),
						Paths: []*rpb.Path{
							{
								BgpPath: &rpb.BGPPath{},
							},
						},
					},
				},
			},
			expected: provider.LookupResult{
				NetMask: 32,
			},
			err: "",
		},
		{
			name: "LPM with default route without more specific, multiple paths",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{
					{
						Pfx: p1.ToProto(),
						Paths: []*rpb.Path{
							// content of first path should be used
							{
								BgpPath: &rpb.BGPPath{
									Communities: []uint32{123},
									LargeCommunities: []*rpb.LargeCommunity{
										{
											DataPart1: 123,
											DataPart2: 456,
										},
									},

									AsPath: []*rpb.ASPathSegment{
										{
											Asns: []uint32{123, 456},
										},
									},
								},
							},
							{
								BgpPath: &rpb.BGPPath{},
							},
						},
					},
				},
			},
			expected: provider.LookupResult{
				ASN:              456,
				ASPath:           []uint32{123, 456},
				Communities:      []uint32{123},
				LargeCommunities: []bgp.LargeCommunity{{LocalData1: 123, LocalData2: 456}},
				NetMask:          0,
			},
			err: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := c.lpmResponseToLookupResult(tc.lpm)
			if err == nil && tc.err != "" {
				t.Errorf("Expected error: %s", tc.err)
			}
			if err != nil {
				if diff := helpers.Diff(err.Error(), tc.err); diff != "" {
					t.Errorf("Error (-got, +want):\n%s", diff)
				}
			}
			if diff := helpers.Diff(result, tc.expected); diff != "" {
				t.Errorf("Result (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestBioRIS(t *testing.T) {
	// Spawn a BMP receiver
	b := server.NewBMPReceiver(server.BMPReceiverConfig{
		KeepalivePeriod: 10 * time.Second,
		AcceptAny:       true,
	})
	defer b.Close()
	if err := b.Listen("127.0.0.1:0"); err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	go b.Serve()

	// Inject some routes
	{
		send := func(t *testing.T, conn net.Conn, pcap string) {
			t.Helper()
			_, err := conn.Write(helpers.ReadPcapL4(t, path.Join("..", "bmp", "testdata", pcap)))
			if err != nil {
				t.Fatalf("Write() error:\n%+v", err)
			}
		}
		bmpConn, err := net.Dial("tcp", b.LocalAddr().String())
		if err != nil {
			t.Fatalf("Dial() error:\n%+v", err)
		}
		defer bmpConn.Close()
		send(t, bmpConn, "bmp-init.pcap")
		send(t, bmpConn, "bmp-peers-up.pcap")
		send(t, bmpConn, "bmp-eor.pcap")
		send(t, bmpConn, "bmp-reach.pcap")
		time.Sleep(20 * time.Millisecond)
	}

	// Check we have our routes
	{
		router := b.GetRouter("127.0.0.1")
		if router == nil {
			t.Fatal("GetRouter() did not return a router")
		}
		vrf := router.GetVRF(0)
		if vrf == nil {
			t.Fatal("GetVRF() did not return a VRF")
		}
		if vrf.IPv4UnicastRIB().RouteCount() != 4 {
			t.Fatalf("IPv4 route count should be 4, not %d", vrf.IPv4UnicastRIB().RouteCount())
		}
		if vrf.IPv6UnicastRIB().RouteCount() != 4 {
			t.Fatalf("IPv6 route count should be 6, not %d", vrf.IPv6UnicastRIB().RouteCount())
		}
	}

	// Prepare BioRIS server
	s := risserver.NewServer(b)
	rpc := grpc.NewServer()
	reflection.Register(rpc)
	pb.RegisterRoutingInformationServiceServer(rpc, s)

	rpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error:\n%+v", err)
	}
	defer rpcListener.Close()
	go rpc.Serve(rpcListener)

	// Check BioRIS server
	{
		risConn, err := grpc.Dial(rpcListener.Addr().String(),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("Dial() error:\n%+v", err)
		}
		defer risConn.Close()
		client := pb.NewRoutingInformationServiceClient(risConn)
		if client == nil {
			t.Fatal("pb.NewRoutingInformationServiceClient() returned nil")
		}
		ipAddr, _ := bnet.IPFromString("2001:db8:1::")
		r, err := client.Get(context.Background(), &pb.GetRequest{
			Router: "127.0.0.1",
			VrfId:  0,
			Pfx:    bnet.NewPfx(ipAddr, 64).ToProto(),
		})
		if err != nil {
			t.Fatalf("Get() error:\n%+v", err)
		}
		if len(r.Routes) == 0 {
			t.Fatal("Get() returned no route")
		}
	}

	// Instantiate provider
	r := reporter.NewMock(t)
	addr := rpcListener.Addr().String()
	config := DefaultConfiguration()
	configP := config.(Configuration)
	configP.RISInstances = []RISInstance{{
		GRPCAddr:   addr,
		GRPCSecure: false,
		VRFId:      0,
	}}
	p, err := configP.New(r, provider.Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, p)

	{
		got, err := p.Lookup(context.Background(),
			netip.MustParseAddr("2001:db8:1::10"),
			netip.Addr{},
			netip.MustParseAddr("2001:db8::7"))
		if err != nil {
			t.Fatalf("Lookup() error:\n%+v", err)
		}
		expected := provider.LookupResult{
			NetMask:     64,
			ASN:         174,
			ASPath:      []uint32{65017, 65013, 174, 174, 174},
			Communities: []uint32{4260954122, 4260954132},
			LargeCommunities: []bgp.LargeCommunity{
				{
					ASN:        65017,
					LocalData1: 300,
					LocalData2: 4,
				},
			},
			NextHop: netip.MustParseAddr("2001:db8::7"),
		}
		if diff := helpers.Diff(got, expected); diff != "" {
			t.Errorf("Lookup() (-got, +want):\n%s", diff)
		}
	}

	for try := 2; try >= 0; try-- {
		gotMetrics := r.GetMetrics("akvorado_inlet_routing_provider_bioris_")
		expectedMetrics := map[string]string{
			// connection_up may take a bit of time
			fmt.Sprintf(`connection_up{ris="%s"}`, addr):                                     "1",
			fmt.Sprintf(`known_routers_total{ris="%s"}`, addr):                               "1",
			fmt.Sprintf(`lpm_request_errors_total{ris="%s",router="127.0.0.1"}`, addr):       "0",
			fmt.Sprintf(`lpm_success_requests_total{ris="%s",router="127.0.0.1"}`, addr):     "1",
			fmt.Sprintf(`lpm_request_timeouts_total{ris="%s",router="127.0.0.1"}`, addr):     "0",
			fmt.Sprintf(`lpm_requests_total{ris="%s",router="127.0.0.1"}`, addr):             "1",
			fmt.Sprintf(`router_agentid_requests_total{ris="%s",router="127.0.0.1"}`, addr):  "0",
			fmt.Sprintf(`router_fallback_requests_total{ris="%s",router="127.0.0.1"}`, addr): "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			if try == 0 {
				t.Fatalf("Metrics (-got, +want):\n%s", diff)
			} else {
				time.Sleep(20 * time.Millisecond)
			}
		} else {
			break
		}
	}
}

func TestNonWorkingBioRIS(t *testing.T) {
	r := reporter.NewMock(t)
	config := DefaultConfiguration()
	configP := config.(Configuration)
	configP.RISInstances = []RISInstance{
		{GRPCAddr: "ris.invalid:1000"},
		{GRPCAddr: "192.0.2.10:1000"},
	}
	p, err := configP.New(r, provider.Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, p)

	gotMetrics := r.GetMetrics("akvorado_inlet_routing_provider_bioris_")
	expectedMetrics := map[string]string{
		`connection_up{ris="ris.invalid:1000"}`: "0",
		`connection_up{ris="192.0.2.10:1000"}`:  "0",
	}
	if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
		t.Errorf("Metrics (-got, +want):\n%s", diff)
	}
}
