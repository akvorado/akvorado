// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package dns

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mdns "github.com/miekg/dns"

	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
)

func newTestComponent(t *testing.T, configuration Configuration) *Component {
	t.Helper()
	r := reporter.NewMock(t)
	c, err := New(r, configuration, Dependencies{Daemon: daemon.NewMock(t)})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	return c
}

func startDNSServer(t *testing.T, handler mdns.HandlerFunc) string {
	t.Helper()
	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket() error:\n%+v", err)
	}
	server := &mdns.Server{
		PacketConn: packetConn,
		Handler:    handler,
	}
	go func() {
		_ = server.ActivateAndServe()
	}()
	t.Cleanup(func() {
		_ = server.Shutdown()
	})
	return packetConn.LocalAddr().String()
}

func ptrResponse(request *mdns.Msg, name string, ttl uint32) *mdns.Msg {
	response := new(mdns.Msg)
	response.SetReply(request)
	response.Authoritative = true
	response.Answer = append(response.Answer, &mdns.PTR{
		Hdr: mdns.RR_Header{
			Name:   request.Question[0].Name,
			Rrtype: mdns.TypePTR,
			Class:  mdns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: name,
	})
	return response
}

func nxdomainResponse(request *mdns.Msg) *mdns.Msg {
	response := new(mdns.Msg)
	response.SetReply(request)
	response.Rcode = mdns.RcodeNameError
	return response
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func testConfig(resolver string) Configuration {
	configuration := DefaultConfiguration()
	configuration.Enabled = true
	configuration.Resolvers = []string{resolver}
	configuration.Timeout = 50 * time.Millisecond
	configuration.Attempts = 1
	configuration.MaxConcurrentQueries = 2
	configuration.Cache.MaxEntries = 10
	configuration.Cache.MinTTL = time.Second
	configuration.Cache.MaxTTL = time.Hour
	configuration.Cache.NegativeTTL = time.Second
	return configuration
}

func TestDisabledLookup(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Enabled = false
	c := newTestComponent(t, configuration)
	if got := c.Lookup(netip.MustParseAddr("192.0.2.1")); got != "" {
		t.Fatalf("Lookup() = %q, expected empty string", got)
	}
	if len(c.queue) != 0 {
		t.Fatalf("queue length = %d, expected 0", len(c.queue))
	}
}

func TestIncludeExcludeSubnets(t *testing.T) {
	var queries atomic.Int32
	resolver := startDNSServer(t, func(w mdns.ResponseWriter, r *mdns.Msg) {
		queries.Add(1)
		_ = w.WriteMsg(ptrResponse(r, "host.example.", 300))
	})
	configuration := testConfig(resolver)
	configuration.IncludeSubnets = []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
	}
	configuration.ExcludeSubnets = []netip.Prefix{
		netip.MustParsePrefix("192.0.2.128/25"),
	}
	c := newTestComponent(t, configuration)
	helpers.StartStop(t, c)

	c.Lookup(netip.MustParseAddr("198.51.100.1"))
	c.Lookup(netip.MustParseAddr("192.0.2.200"))
	c.Lookup(netip.MustParseAddr("192.0.2.10"))

	waitFor(t, func() bool { return queries.Load() == 1 })
}

func TestCacheHit(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Enabled = true
	c := newTestComponent(t, configuration)
	c.putCached(normalizeAddr(netip.MustParseAddr("192.0.2.1")), "router.example", time.Minute)

	if got := c.Lookup(netip.MustParseAddr("192.0.2.1")); got != "router.example" {
		t.Fatalf("Lookup() = %q, expected router.example", got)
	}
}

func TestCacheMissDoesNotBlock(t *testing.T) {
	resolver := startDNSServer(t, func(w mdns.ResponseWriter, r *mdns.Msg) {
		time.Sleep(200 * time.Millisecond)
		_ = w.WriteMsg(ptrResponse(r, "slow.example.", 300))
	})
	configuration := testConfig(resolver)
	configuration.Timeout = time.Second
	c := newTestComponent(t, configuration)
	helpers.StartStop(t, c)

	start := time.Now()
	if got := c.Lookup(netip.MustParseAddr("192.0.2.1")); got != "" {
		t.Fatalf("Lookup() = %q, expected empty string", got)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("Lookup() took %s, expected non-blocking miss", elapsed)
	}
}

func TestInitialLookupWaitReturnsPositiveResult(t *testing.T) {
	configuration := testConfig("127.0.0.1:53")
	configuration.WaitForInitialResult = true
	configuration.InitialTimeout = 200 * time.Millisecond
	c := newTestComponent(t, configuration)
	c.lookup = func(netip.Addr) (string, time.Duration, bool, error) {
		time.Sleep(20 * time.Millisecond)
		return "initial.example.", time.Minute, false, nil
	}
	helpers.StartStop(t, c)

	if got := c.Lookup(netip.MustParseAddr("192.0.2.1")); got != "initial.example" {
		t.Fatalf("Lookup() = %q, expected initial.example", got)
	}
}

func TestInitialLookupWaitTimesOutThenCacheHit(t *testing.T) {
	configuration := testConfig("127.0.0.1:53")
	configuration.WaitForInitialResult = true
	configuration.InitialTimeout = 20 * time.Millisecond
	c := newTestComponent(t, configuration)
	c.lookup = func(netip.Addr) (string, time.Duration, bool, error) {
		time.Sleep(100 * time.Millisecond)
		return "later.example.", time.Minute, false, nil
	}
	helpers.StartStop(t, c)

	ip := netip.MustParseAddr("192.0.2.1")
	start := time.Now()
	if got := c.Lookup(ip); got != "" {
		t.Fatalf("Lookup() = %q, expected empty string", got)
	}
	if elapsed := time.Since(start); elapsed > 80*time.Millisecond {
		t.Fatalf("Lookup() took %s, expected initial timeout", elapsed)
	}
	waitFor(t, func() bool { return c.Lookup(ip) == "later.example" })
}

func TestInitialLookupWaitSharesPendingQuery(t *testing.T) {
	var queries atomic.Int32
	configuration := testConfig("127.0.0.1:53")
	configuration.WaitForInitialResult = true
	configuration.InitialTimeout = 200 * time.Millisecond
	c := newTestComponent(t, configuration)
	c.lookup = func(netip.Addr) (string, time.Duration, bool, error) {
		queries.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "shared.example.", time.Minute, false, nil
	}
	helpers.StartStop(t, c)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if got := c.Lookup(netip.MustParseAddr("192.0.2.1")); got != "shared.example" {
				t.Errorf("Lookup() = %q, expected shared.example", got)
			}
		})
	}
	wg.Wait()
	if got := queries.Load(); got != 1 {
		t.Fatalf("queries = %d, expected 1", got)
	}
}

func TestInitialLookupWaitReturnsNegativeResult(t *testing.T) {
	var queries atomic.Int32
	configuration := testConfig("127.0.0.1:53")
	configuration.WaitForInitialResult = true
	configuration.InitialTimeout = 200 * time.Millisecond
	c := newTestComponent(t, configuration)
	c.lookup = func(netip.Addr) (string, time.Duration, bool, error) {
		queries.Add(1)
		time.Sleep(20 * time.Millisecond)
		return "", 0, true, nil
	}
	helpers.StartStop(t, c)

	ip := netip.MustParseAddr("192.0.2.1")
	if got := c.Lookup(ip); got != "" {
		t.Fatalf("Lookup() = %q, expected empty string", got)
	}
	if got := c.Lookup(ip); got != "" {
		t.Fatalf("Lookup() = %q, expected negative cache hit", got)
	}
	if got := queries.Load(); got != 1 {
		t.Fatalf("queries = %d, expected negative cache hit", got)
	}
}

func TestQueueFullClearsPendingState(t *testing.T) {
	configuration := testConfig("127.0.0.1:53")
	configuration.MaxConcurrentQueries = 1
	c := newTestComponent(t, configuration)
	for i := range cap(c.queue) {
		c.queue <- netip.AddrFrom4([4]byte{192, 0, 2, byte(i + 1)})
	}

	if got := c.Lookup(netip.MustParseAddr("192.0.2.200")); got != "" {
		t.Fatalf("Lookup() = %q, expected empty string", got)
	}
	c.pendingMu.Lock()
	pending := len(c.pending)
	c.pendingMu.Unlock()
	if pending != 0 {
		t.Fatalf("pending entries = %d, expected 0", pending)
	}
}

func TestNegativeCache(t *testing.T) {
	var queries atomic.Int32
	resolver := startDNSServer(t, func(w mdns.ResponseWriter, r *mdns.Msg) {
		queries.Add(1)
		_ = w.WriteMsg(nxdomainResponse(r))
	})
	c := newTestComponent(t, testConfig(resolver))
	helpers.StartStop(t, c)

	ip := netip.MustParseAddr("192.0.2.1")
	c.Lookup(ip)
	waitFor(t, func() bool { return queries.Load() == 1 })
	c.Lookup(ip)
	time.Sleep(50 * time.Millisecond)
	if got := queries.Load(); got != 1 {
		t.Fatalf("queries = %d, expected negative cache hit", got)
	}
}

func TestTTLExpiry(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Enabled = true
	c := newTestComponent(t, configuration)
	ip := normalizeAddr(netip.MustParseAddr("192.0.2.1"))
	c.putCached(ip, "expired.example", 20*time.Millisecond)
	waitFor(t, func() bool {
		return c.Lookup(ip) == ""
	})
}

func TestTrimSuffixes(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.Enabled = true
	configuration.TrimSuffixes = []string{".lan.", "example.net"}
	c := newTestComponent(t, configuration)

	if got := c.normalizeName("host.lan."); got != "host" {
		t.Fatalf("normalizeName() = %q, expected host", got)
	}
	if got := c.normalizeName("router.example.net."); got != "router" {
		t.Fatalf("normalizeName() = %q, expected router", got)
	}
}

func TestSingleflight(t *testing.T) {
	var queries atomic.Int32
	c := newTestComponent(t, testConfig("127.0.0.1:53"))
	c.lookup = func(netip.Addr) (string, time.Duration, bool, error) {
		queries.Add(1)
		time.Sleep(100 * time.Millisecond)
		return "single.example.", time.Minute, false, nil
	}
	helpers.StartStop(t, c)

	for range 20 {
		c.Lookup(netip.MustParseAddr("192.0.2.1"))
	}
	waitFor(t, func() bool { return queries.Load() == 1 })
}

func TestConcurrencyLimit(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32
	var queries atomic.Int32
	configuration := testConfig("127.0.0.1:53")
	configuration.MaxConcurrentQueries = 1
	c := newTestComponent(t, configuration)
	c.lookup = func(netip.Addr) (string, time.Duration, bool, error) {
		current := active.Add(1)
		for {
			previous := maxActive.Load()
			if current <= previous || maxActive.CompareAndSwap(previous, current) {
				break
			}
		}
		queries.Add(1)
		time.Sleep(100 * time.Millisecond)
		active.Add(-1)
		return fmt.Sprintf("host-%d.example.", queries.Load()), time.Minute, false, nil
	}
	helpers.StartStop(t, c)

	c.Lookup(netip.MustParseAddr("192.0.2.1"))
	c.Lookup(netip.MustParseAddr("192.0.2.2"))
	waitFor(t, func() bool { return queries.Load() == 2 })
	if got := maxActive.Load(); got > 1 {
		t.Fatalf("max active queries = %d, expected <= 1", got)
	}
}

func TestIPv4AndIPv6PTR(t *testing.T) {
	resolver := startDNSServer(t, func(w mdns.ResponseWriter, r *mdns.Msg) {
		switch r.Question[0].Name {
		case "1.2.0.192.in-addr.arpa.":
			_ = w.WriteMsg(ptrResponse(r, "ipv4.example.", 300))
		case "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.":
			_ = w.WriteMsg(ptrResponse(r, "ipv6.example.", 300))
		default:
			_ = w.WriteMsg(nxdomainResponse(r))
		}
	})
	c := newTestComponent(t, testConfig(resolver))
	helpers.StartStop(t, c)

	ipv4 := netip.MustParseAddr("192.0.2.1")
	ipv6 := netip.MustParseAddr("2001:db8::1")
	c.Lookup(ipv4)
	c.Lookup(ipv6)
	waitFor(t, func() bool {
		return c.Lookup(ipv4) == "ipv4.example" && c.Lookup(ipv6) == "ipv6.example"
	})
}
