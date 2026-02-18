// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"net"
	"net/netip"
	"path"
	"slices"
	"strconv"
	"testing"
	"time"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/outlet/routing/provider"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

func TestBMP(t *testing.T) {
	dial := func(t *testing.T, c *Provider) net.Conn {
		t.Helper()
		conn, err := net.Dial("tcp", c.LocalAddr().String())
		if err != nil {
			t.Fatalf("Dial() error:\n%+v", err)
		}
		t.Cleanup(func() {
			conn.Close()
		})
		return conn
	}
	send := func(t *testing.T, conn net.Conn, pcap string) {
		t.Helper()
		_, err := conn.Write(helpers.ReadPcapL4(t, path.Join("testdata", pcap)))
		if err != nil {
			t.Fatalf("Write() error:\n%+v", err)
		}
	}
	dumpRIB := func(t *testing.T, c *Provider) map[netip.Addr][]string {
		t.Helper()
		c.mu.RLock()
		defer c.mu.RUnlock()
		result := map[netip.Addr][]string{}
		for prefix, prefixIdx := range c.rib.tree.All() {
			for route := range c.rib.iterateRoutesForPrefixIndex(prefixIdx) {
				nlriRef := c.rib.nlris.Get(route.nlri)
				nh := c.rib.nextHops.Get(route.nextHop)
				attrs := c.rib.rtas.Get(route.attributes)
				var peer netip.Addr
				for pkey, pinfo := range c.peers {
					if pinfo.reference == route.peer {
						peer = pkey.ip
						break
					}
				}
				if _, ok := result[peer.Unmap()]; !ok {
					result[peer.Unmap()] = []string{}
				}
				prefix = helpers.UnmapPrefix(prefix)
				peer = peer.Unmap()
				result[peer] = append(result[peer],
					fmt.Sprintf("[%s] %s via %s %s/%d %d %v %v %v",
						nlriRef.family,
						prefix, netip.Addr(nh).Unmap(),
						nlriRef.rd,
						nlriRef.path,
						attrs.asn, attrs.asPath,
						attrs.communities, attrs.largeCommunities))
				slices.Sort(result[peer])
			}
		}
		return result
	}

	// The pcap files are extracted from BMP session from a Juniper vMX. See:
	// https://github.com/vincentbernat/network-lab/tree/master/lab-juniper-vmx-bmp

	t.Run("init, terminate", func(t *testing.T) {
		r := reporter.NewMock(t)
		p, mockClock := NewMock(t, r, DefaultConfiguration())
		helpers.StartStop(t, p)
		conn := dial(t, p)

		// Init+EOR
		send(t, conn, "bmp-init.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "0",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "0",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "0",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "0",
			`routes{exporter="127.0.0.1"}`:                                                "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		send(t, conn, "bmp-terminate.pcap")
		time.Sleep(30 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics = map[string]string{
			`closed_connections_total{exporter="127.0.0.1"}`:                              "1",
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "0",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "0",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "0",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "1",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "0",
			`routes{exporter="127.0.0.1"}`:                                                "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
		for i := 0; ; i++ {
			if _, err := conn.Write([]byte{1}); err != nil {
				break
			} else if i >= 200 {
				t.Fatal("Write() did not error while connection should be closed")
			}
			time.Sleep(5 * time.Millisecond)
		}

		time.Sleep(20 * time.Millisecond)
		mockClock.Add(2 * time.Hour)
		for tries := 20; tries >= 0; tries-- {
			time.Sleep(5 * time.Millisecond)
			gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
			expectedMetrics = map[string]string{
				`closed_connections_total{exporter="127.0.0.1"}`:                              "1",
				`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
				`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
				`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "0",
				`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
				`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "0",
				`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "0",
				`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "1",
				`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
				`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
				`peers{exporter="127.0.0.1"}`:                                                 "0",
				`routes{exporter="127.0.0.1"}`:                                                "0",
			}
			if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
				if tries > 0 {
					continue
				}
				t.Errorf("Metrics (-got, +want):\n%s", diff)
			}
			break
		}
	})

	t.Run("init, peers up, eor", func(t *testing.T) {
		r := reporter.NewMock(t)
		p, _ := NewMock(t, r, DefaultConfiguration())
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "8",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, eor, reach NLRI", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-reach-addpath.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "26",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "18",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("2001:db8::3"): {
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::3 0:0/0 174 [65013 65013 174 174 174] [4260691978 4260691988] []",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::3 0:0/0 12322 [65013 65013 1299 1299 1299 12322] [4260691998] []",
				"[ipv6-unicast] 2001:db8::2/127 via 2001:db8::3 0:0/0 65013 [65013] [] []",
			},
			netip.MustParseAddr("2001:db8::7"): {
				"[ipv4-unicast] 192.0.2.6/31 via 192.0.2.7 0:0/0 65017 [65017] [] []",
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::7 0:0/0 174 [65017 65013 174 174 174] [4260954122 4260954132] [{65017 300 4}]",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::7 0:0/0 12322 [65017 65017 1299 1299 1299 12322] [4260954142] [{65017 400 2}]",
				"[ipv6-unicast] 2001:db8::6/127 via 2001:db8::7 0:0/0 65017 [65017] [] []",
				"[l2vpn-evpn] 198.51.100.0/26 via 2001:db8::7 65017:104/0 64476 [65017 65017 3356 64476] [4260955215] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/25 via 192.0.2.7 65017:101/0 64476 [65017 65017 174 1299 64476] [4260954122 4260954132] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/25 via 192.0.2.7 65017:102/0 64476 [65017 65017 174 3356 3356 3356 64476] [4260954122 4260954132] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/26 via 192.0.2.7 65017:103/0 64476 [65017 65017 3356 64476] [4260955215] []",
				"[l3vpn-ipv4-unicast] 198.51.100.128/25 via 192.0.2.7 65017:101/0 396919 [65017 65017 174 29447 396919] [4260954124] []",
				"[l3vpn-ipv4-unicast] 198.51.100.128/25 via 192.0.2.7 65017:102/0 396919 [65017 65017 6453 396919] [4260954131] []",
				"[l3vpn-ipv6-unicast] 2001:db8:4::/64 via 2001:db8::7 65017:101/0 29447 [65017 65017 1299 1299 1299 29447] [4260954412] []",
			},
			netip.MustParseAddr("192.0.2.1"): {
				"[ipv4-unicast] 192.0.2.0/31 via 192.0.2.1 0:0/0 65011 [65011] [] []",
				"[ipv4-unicast] 198.51.100.0/25 via 192.0.2.1 0:0/0 64476 [65011 65011 174 1299 64476] [4260560906 4260560916] []",
				"[ipv4-unicast] 198.51.100.128/25 via 192.0.2.1 0:0/0 396919 [65011 65011 174 29447 396919] [4260560908] []",
			},
			netip.MustParseAddr("192.0.2.5"): {
				"[ipv4-unicast] 192.0.2.4/31 via 192.0.2.5 0:0/1 65500 [] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, no peers up, eor, reach NLRI", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-reach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			// Same metrics as previously, except the AddPath peer.
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "0",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "17",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "0",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "3",
			`routes{exporter="127.0.0.1"}`:                                                "17",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, no peers up, eor, reach NLRI, peers up", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		c, _ := NewMock(t, r, config)
		helpers.StartStop(t, c)
		conn := dial(t, c)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "25",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "17",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, eor, reach NLRI, 1 peer down", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-peer-down.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "25",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "5",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "3",
			`routes{exporter="127.0.0.1"}`:                                                "14",
			`removed_peers_total{exporter="127.0.0.1"}`:                                   "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("2001:db8::3"): {
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::3 0:0/0 174 [65013 65013 174 174 174] [4260691978 4260691988] []",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::3 0:0/0 12322 [65013 65013 1299 1299 1299 12322] [4260691998] []",
				"[ipv6-unicast] 2001:db8::2/127 via 2001:db8::3 0:0/0 65013 [65013] [] []",
			},
			netip.MustParseAddr("2001:db8::7"): {
				"[ipv4-unicast] 192.0.2.6/31 via 192.0.2.7 0:0/0 65017 [65017] [] []",
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::7 0:0/0 174 [65017 65013 174 174 174] [4260954122 4260954132] [{65017 300 4}]",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::7 0:0/0 12322 [65017 65017 1299 1299 1299 12322] [4260954142] [{65017 400 2}]",
				"[ipv6-unicast] 2001:db8::6/127 via 2001:db8::7 0:0/0 65017 [65017] [] []",
				"[l2vpn-evpn] 198.51.100.0/26 via 2001:db8::7 65017:104/0 64476 [65017 65017 3356 64476] [4260955215] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/25 via 192.0.2.7 65017:101/0 64476 [65017 65017 174 1299 64476] [4260954122 4260954132] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/25 via 192.0.2.7 65017:102/0 64476 [65017 65017 174 3356 3356 3356 64476] [4260954122 4260954132] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/26 via 192.0.2.7 65017:103/0 64476 [65017 65017 3356 64476] [4260955215] []",
				"[l3vpn-ipv4-unicast] 198.51.100.128/25 via 192.0.2.7 65017:101/0 396919 [65017 65017 174 29447 396919] [4260954124] []",
				"[l3vpn-ipv4-unicast] 198.51.100.128/25 via 192.0.2.7 65017:102/0 396919 [65017 65017 6453 396919] [4260954131] []",
				"[l3vpn-ipv6-unicast] 2001:db8:4::/64 via 2001:db8::7 65017:101/0 29447 [65017 65017 1299 1299 1299 29447] [4260954412] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("only accept RD 65017:104", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.RDs = []RD{MustParseRD("65017:104")}
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "25",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("2001:db8::7"): {
				"[l2vpn-evpn] 198.51.100.0/26 via 2001:db8::7 65017:104/0 64476 [65017 65017 3356 64476] [4260955215] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("only accept RD 0:0", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.RDs = []RD{MustParseRD("0:0")}
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "25",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "10",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("2001:db8::3"): {
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::3 0:0/0 174 [65013 65013 174 174 174] [4260691978 4260691988] []",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::3 0:0/0 12322 [65013 65013 1299 1299 1299 12322] [4260691998] []",
				"[ipv6-unicast] 2001:db8::2/127 via 2001:db8::3 0:0/0 65013 [65013] [] []",
			},
			netip.MustParseAddr("2001:db8::7"): {
				"[ipv4-unicast] 192.0.2.6/31 via 192.0.2.7 0:0/0 65017 [65017] [] []",
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::7 0:0/0 174 [65017 65013 174 174 174] [4260954122 4260954132] [{65017 300 4}]",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::7 0:0/0 12322 [65017 65017 1299 1299 1299 12322] [4260954142] [{65017 400 2}]",
				"[ipv6-unicast] 2001:db8::6/127 via 2001:db8::7 0:0/0 65017 [65017] [] []",
			},
			netip.MustParseAddr("192.0.2.1"): {
				"[ipv4-unicast] 192.0.2.0/31 via 192.0.2.1 0:0/0 65011 [65011] [] []",
				"[ipv4-unicast] 198.51.100.0/25 via 192.0.2.1 0:0/0 64476 [65011 65011 174 1299 64476] [4260560906 4260560916] []",
				"[ipv4-unicast] 198.51.100.128/25 via 192.0.2.1 0:0/0 396919 [65011 65011 174 29447 396919] [4260560908] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, eor, reach, unreach", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.RDs = []RD{MustParseRD("0:0")}
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-unreach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "33",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "1",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "3",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "1",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "1",
			`routes{exporter="127.0.0.1"}`:                                                "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [65019] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [65019 65019 64476] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, filtering on 65500:108", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.RDs = []RD{MustParseRD("65500:108")}
		c, _ := NewMock(t, r, configP)
		helpers.StartStop(t, c)
		conn := dial(t, c)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "routes")
		expectedMetrics := map[string]string{
			`routes{exporter="127.0.0.1"}`: "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, filtering on 65500:110", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.RDs = []RD{MustParseRD("65500:110")}
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "routes")
		expectedMetrics := map[string]string{
			`routes{exporter="127.0.0.1"}`: "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, do not collect AS paths or communities", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.CollectCommunities = false
		configP.CollectASPaths = false
		c, _ := NewMock(t, r, configP)
		helpers.StartStop(t, c)
		conn := dial(t, c)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "routes")
		expectedMetrics := map[string]string{
			`routes{exporter="127.0.0.1"}`: "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [] [] []",
			},
		}
		gotRIB := dumpRIB(t, c)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, do not collect ASNs", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.CollectASNs = false
		configP.CollectCommunities = false
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "routes")
		expectedMetrics := map[string]string{
			`routes{exporter="127.0.0.1"}`: "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 0 [65019] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 0 [65019 65019 64476] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, eor, unreach", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-unreach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "16",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "0",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, eor, reach, unreach×2", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.CollectASNs = false
		configP.CollectASPaths = false
		configP.CollectCommunities = false
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-unreach.pcap")
		send(t, conn, "bmp-unreach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "41",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("2001:db8::7"): {
				// This route stays because we tweaked it in reach.pcap
				"[l2vpn-evpn] 198.51.100.0/26 via 2001:db8::7 65017:104/0 0 [] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, eor, reach×2, unreach", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-eor.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-unreach.pcap")
		send(t, conn, "bmp-unreach.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "41",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, peers up, reach, eor", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.CollectASPaths = false
		configP.CollectCommunities = false
		p, _ := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-eor.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "4",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "25",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "4",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "4",
			`routes{exporter="127.0.0.1"}`:                                                "17",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("2001:db8::3"): {
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::3 0:0/0 174 [] [] []",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::3 0:0/0 12322 [] [] []",
				"[ipv6-unicast] 2001:db8::2/127 via 2001:db8::3 0:0/0 65013 [] [] []",
			},
			netip.MustParseAddr("2001:db8::7"): {
				"[ipv4-unicast] 192.0.2.6/31 via 192.0.2.7 0:0/0 65017 [] [] []",
				"[ipv6-unicast] 2001:db8:1::/64 via 2001:db8::7 0:0/0 174 [] [] []",
				"[ipv6-unicast] 2001:db8:2::/64 via 2001:db8::7 0:0/0 12322 [] [] []",
				"[ipv6-unicast] 2001:db8::6/127 via 2001:db8::7 0:0/0 65017 [] [] []",
				"[l2vpn-evpn] 198.51.100.0/26 via 2001:db8::7 65017:104/0 64476 [] [] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/25 via 192.0.2.7 65017:101/0 64476 [] [] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/25 via 192.0.2.7 65017:102/0 64476 [] [] []",
				"[l3vpn-ipv4-unicast] 198.51.100.0/26 via 192.0.2.7 65017:103/0 64476 [] [] []",
				"[l3vpn-ipv4-unicast] 198.51.100.128/25 via 192.0.2.7 65017:101/0 396919 [] [] []",
				"[l3vpn-ipv4-unicast] 198.51.100.128/25 via 192.0.2.7 65017:102/0 396919 [] [] []",
				"[l3vpn-ipv6-unicast] 2001:db8:4::/64 via 2001:db8::7 65017:101/0 29447 [] [] []",
			},
			netip.MustParseAddr("192.0.2.1"): {
				"[ipv4-unicast] 192.0.2.0/31 via 192.0.2.1 0:0/0 65011 [] [] []",
				"[ipv4-unicast] 198.51.100.0/25 via 192.0.2.1 0:0/0 64476 [] [] []",
				"[ipv4-unicast] 198.51.100.128/25 via 192.0.2.1 0:0/0 396919 [] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, connection down", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.CollectASPaths = false
		configP.CollectCommunities = false
		p, mockClock := NewMock(t, r, configP)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		conn.Close()
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "1",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "3",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "1",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "1",
			`routes{exporter="127.0.0.1"}`:                                                "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}

		mockClock.Add(2 * time.Hour)
		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics = map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "1",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "3",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "1",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "0",
			`routes{exporter="127.0.0.1"}`:                                                "0",
			`removed_peers_total{exporter="127.0.0.1"}`:                                   "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB = map[netip.Addr][]string{}
		gotRIB = dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, unknown family, reach", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		send(t, conn, "bmp-reach-unknown-family.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		ignoredMetric := `ignored_updates_total{error="afi-safi",exporter="127.0.0.1"}`
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "1",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "4",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "1",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "1",
			`routes{exporter="127.0.0.1"}`:                                                "2",
			ignoredMetric:                                                                 "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [65019] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [65019 65019 64476] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, unhandled family, reach", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-l3vpn.pcap")
		send(t, conn, "bmp-reach-vpls.pcap")
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "1",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "1",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "4",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "1",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "2",
			`routes{exporter="127.0.0.1"}`:                                                "2",
			`ignored_nlri_total{exporter="127.0.0.1",type="l2vpn-vpls"}`:                  "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
	})

	t.Run("init, l3vpn peer, init, l3vpn peer, connection down, terminate", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		configP := config.(Configuration)
		configP.CollectASPaths = false
		configP.CollectCommunities = false
		p, mockClock := NewMock(t, r, configP)
		helpers.StartStop(t, p)

		conn1 := dial(t, p)
		send(t, conn1, "bmp-init.pcap")
		send(t, conn1, "bmp-l3vpn.pcap")
		conn2 := dial(t, p)
		send(t, conn2, "bmp-init.pcap")
		send(t, conn2, "bmp-l3vpn.pcap")
		conn1.Close()
		time.Sleep(20 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics := map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "2",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "2",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "6",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "2",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "2",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "2",
			`routes{exporter="127.0.0.1"}`:                                                "4",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB := map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [] [] []",
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [] [] []",
			},
		}
		gotRIB := dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}

		mockClock.Add(2 * time.Hour)
		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics = map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "2",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "2",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "6",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "2",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "0",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "2",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "1",
			`peers{exporter="127.0.0.1"}`:                                                 "1",
			`routes{exporter="127.0.0.1"}`:                                                "2",
			`removed_peers_total{exporter="127.0.0.1"}`:                                   "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}

		expectedRIB = map[netip.Addr][]string{
			netip.MustParseAddr("192.0.2.9"): {
				"[ipv4-unicast] 192.0.2.8/31 via 192.0.2.9 65500:108/0 65019 [] [] []",
				"[ipv4-unicast] 198.51.100.0/29 via 192.0.2.9 65500:108/0 64476 [] [] []",
			},
		}
		gotRIB = dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}

		send(t, conn2, "bmp-terminate.pcap")
		time.Sleep(30 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics = map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "2",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "2",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "6",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "2",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "1",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "2",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "2",
			`peers{exporter="127.0.0.1"}`:                                                 "1",
			`routes{exporter="127.0.0.1"}`:                                                "2",
			`removed_peers_total{exporter="127.0.0.1"}`:                                   "1",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
		gotRIB = dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}

		mockClock.Add(2 * time.Hour)
		time.Sleep(20 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "-locked_duration", "-buffer_size", "-message_queue")
		expectedMetrics = map[string]string{
			`received_messages_total{exporter="127.0.0.1",type="initiation"}`:             "2",
			`received_messages_total{exporter="127.0.0.1",type="peer-down-notification"}`: "0",
			`received_messages_total{exporter="127.0.0.1",type="peer-up-notification"}`:   "2",
			`received_messages_total{exporter="127.0.0.1",type="route-mirroring"}`:        "0",
			`received_messages_total{exporter="127.0.0.1",type="route-monitoring"}`:       "6",
			`received_messages_total{exporter="127.0.0.1",type="statistics-report"}`:      "2",
			`received_messages_total{exporter="127.0.0.1",type="termination"}`:            "1",
			`received_messages_total{exporter="127.0.0.1",type="unknown"}`:                "0",
			`opened_connections_total{exporter="127.0.0.1"}`:                              "2",
			`closed_connections_total{exporter="127.0.0.1"}`:                              "2",
			`peers{exporter="127.0.0.1"}`:                                                 "0",
			`routes{exporter="127.0.0.1"}`:                                                "0",
			`removed_peers_total{exporter="127.0.0.1"}`:                                   "2",
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics (-got, +want):\n%s", diff)
		}
		expectedRIB = map[netip.Addr][]string{}
		gotRIB = dumpRIB(t, p)
		if diff := helpers.Diff(gotRIB, expectedRIB); diff != "" {
			t.Errorf("RIB (-got, +want):\n%s", diff)
		}
	})

	t.Run("lookup", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		send(t, conn, "bmp-peers-up.pcap")
		send(t, conn, "bmp-reach.pcap")
		send(t, conn, "bmp-eor.pcap")
		time.Sleep(20 * time.Millisecond)

		lookup, _ := p.Lookup(t.Context(),
			netip.MustParseAddr("2001:db8:1::10"),
			netip.MustParseAddr("2001:db8::a"), netip.Addr{})
		if lookup.ASN != 174 {
			t.Errorf("Lookup() == %d, expected 174", lookup.ASN)
		}

		// Add another prefix
		p.rib.AddPrefix(netip.MustParsePrefix("2001:db8:1::/64"), route{
			peer:       1,
			nlri:       p.rib.nlris.Put(nlri{family: bgp.RF_IPv4_UC}),
			nextHop:    p.rib.nextHops.Put(nextHop(netip.MustParseAddr("2001:db8::a"))),
			attributes: p.rib.rtas.Put(routeAttributes{asn: 176}),
		})

		lookup, _ = p.Lookup(t.Context(),
			netip.MustParseAddr("2001:db8:1::10"),
			netip.MustParseAddr("2001:db8::a"), netip.Addr{})
		if lookup.ASN != 176 {
			t.Errorf("Lookup() == %d, expected 176", lookup.ASN)
		}
		lookup, _ = p.Lookup(t.Context(),
			netip.MustParseAddr("2001:db8:1::10"),
			netip.MustParseAddr("2001:db8::b"), netip.Addr{})
		if lookup.ASN != 174 {
			t.Errorf("Lookup() == %d, expected 174", lookup.ASN)
		}
	})

	t.Run("populate", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		p.PopulateRIB(t)

		lookup, _ := p.Lookup(t.Context(),
			netip.MustParseAddr("::ffff:192.0.2.2"),
			netip.MustParseAddr("::ffff:198.51.100.200"), netip.Addr{})
		if lookup.ASN != 174 {
			t.Errorf("Lookup() == %d, expected 174", lookup.ASN)
		}
		lookup, _ = p.Lookup(t.Context(),
			netip.MustParseAddr("::ffff:192.0.2.254"),
			netip.MustParseAddr("::ffff:198.51.100.200"), netip.Addr{})
		if lookup.ASN != 0 {
			t.Errorf("Lookup() == %d, expected 0", lookup.ASN)
		}
	})

	t.Run("LPM test", func(t *testing.T) {
		r := reporter.NewMock(t)
		config := DefaultConfiguration()
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		p.PopulateRIB(t)

		// Despite having the "wrong" nexthop, we select the more specific one.
		// This is not optimal, but performance is better this way and on a
		// proper network, all routers should know the more specific.
		lookup, _ := p.Lookup(t.Context(),
			netip.MustParseAddr("::ffff:192.168.145.10"),
			netip.MustParseAddr("::ffff:203.0.113.14"), netip.Addr{})
		expected := provider.LookupResult{
			ASN:     1234,
			ASPath:  []uint32{1234},
			NetMask: 22,
			NextHop: netip.MustParseAddr("::ffff:203.0.113.15"),
		}
		if diff := helpers.Diff(lookup, expected); diff != "" {
			t.Errorf("Lookup() (-got, +want):\n%s", diff)
		}
	})

	t.Run("check buffer size", func(t *testing.T) {
		// Without
		r := reporter.NewMock(t)
		config := DefaultConfiguration().(Configuration)
		p, _ := NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn := dial(t, p)

		send(t, conn, "bmp-init.pcap")
		time.Sleep(10 * time.Millisecond)
		gotMetrics := r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "buffer_size")
		bufferSize := gotMetrics[`buffer_size_bytes{exporter="127.0.0.1"}`]
		bufferSize1, _ := strconv.ParseFloat(bufferSize, 32)

		// With
		r = reporter.NewMock(t)
		config = DefaultConfiguration().(Configuration)
		config.ReceiveBuffer = 100_000_000
		p, _ = NewMock(t, r, config)
		helpers.StartStop(t, p)
		conn = dial(t, p)

		send(t, conn, "bmp-init.pcap")
		time.Sleep(10 * time.Millisecond)
		gotMetrics = r.GetMetrics("akvorado_outlet_routing_provider_bmp_", "buffer_size")
		bufferSize = gotMetrics[`buffer_size_bytes{exporter="127.0.0.1"}`]
		bufferSize2, _ := strconv.ParseFloat(bufferSize, 32)

		if bufferSize2 < bufferSize1 {
			t.Fatalf("Buffer size was unchanged (%f <= %f)", bufferSize1, bufferSize2)
		}
	})
}
