package bioris

import (
	"net/netip"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/inlet/bmp"

	pb "github.com/bio-routing/bio-rd/cmd/ris/api"
	bnet "github.com/bio-routing/bio-rd/net"
	rpb "github.com/bio-routing/bio-rd/route/api"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	"golang.org/x/exp/slices"
)

func TestChooseRouter(t *testing.T) {
	// we test the method to choose an appropriate router/ris for lookup here
	r := reporter.NewMock(t)
	// first step: Mock a component
	c := &Component{
		r:      r,
		i:      make(map[string]*RISInstanceRuntime),
		router: make(map[netip.Addr][]*RISInstanceRuntime),
	}

	c.initMetrics()

	// first test: we have no routers/ris instances and fail with an error
	t.Run("no router", func(t *testing.T) {
		expected := "no applicable router found for bio flow lookup"
		_, _, err := c.chooseRouter(netip.MustParseAddr("10.0.0.0"))
		if diff := helpers.Diff(err.Error(), expected); diff != "" {
			t.Errorf("Error (-got, +want):\n%s", diff)
		}
	})

	// create some RisInstanceRuntime objects
	ris1 := &RISInstanceRuntime{config: RISInstance{GRPCAddr: "ris1"}}
	ris2 := &RISInstanceRuntime{config: RISInstance{GRPCAddr: "ris2"}}
	ris3 := &RISInstanceRuntime{config: RISInstance{GRPCAddr: "ris3"}}

	// add them to the component
	c.i["ris1"] = ris1
	c.i["ris2"] = ris2
	c.i["ris3"] = ris3

	// create a few routers
	r1 := netip.MustParseAddr("10.0.0.1")
	r2 := netip.MustParseAddr("10.0.0.2")
	r3 := netip.MustParseAddr("10.0.0.3")
	r4 := netip.MustParseAddr("10.0.0.4")
	r5 := netip.MustParseAddr("10.0.0.5")

	// add routers to the ris components
	// r1 is on ris1 and ris3
	c.router[r1] = []*RISInstanceRuntime{ris1, ris3}
	// r2 is on ris2
	c.router[r2] = []*RISInstanceRuntime{ris2}
	// r3 is on ris1 and ris3
	c.router[r3] = []*RISInstanceRuntime{ris1, ris3}
	// r4 is on ris2
	c.router[r4] = []*RISInstanceRuntime{ris2}
	// r5 is on ris1
	c.router[r5] = []*RISInstanceRuntime{ris1}

	// test exact match for r1
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
		// check if ris is in the list of expected ris instances, if not fail
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
	c := &Component{
		r: r,
	}

	// create some bnet prefixes
	p1Ip, _ := bnet.IPFromString("::")
	p1 := bnet.NewPfx(p1Ip, 0)
	p2Ip, _ := bnet.IPFromString("2001:db8::")
	p2 := bnet.NewPfx(p2Ip, 32)

	cases := []struct {
		name     string
		lpm      *pb.LPMResponse
		expected bmp.LookupResult
		err      string
	}{
		{
			name:     "LPM without route",
			lpm:      &pb.LPMResponse{},
			expected: bmp.LookupResult{},
			err:      "lpm: no route returned",
		},
		{
			name:     "LPM is nil",
			lpm:      nil,
			expected: bmp.LookupResult{},
			err:      "lpm: result empty",
		},
		{
			name: "LPM without path",
			lpm: &pb.LPMResponse{
				Routes: []*rpb.Route{},
			},
			expected: bmp.LookupResult{},
			err:      "lpm: no route returned",
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
			expected: bmp.LookupResult{},
			err:      "lpm: no path found",
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
			expected: bmp.LookupResult{},
			err:      "lpm: path has no bgp path",
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
			expected: bmp.LookupResult{
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
			expected: bmp.LookupResult{
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
			expected: bmp.LookupResult{
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
