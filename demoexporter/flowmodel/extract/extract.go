// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/bits"
	"net/netip"
	"slices"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/demoexporter/flowmodel"
)

// Result is the output of the extraction: the model metadata and the training
// examples.
type Result struct {
	Metadata flowmodel.Metadata
	Examples []flowmodel.Example
	// Flows is the number of flows read from ClickHouse.
	Flows int
}

// prefixAgg accumulates the statistics of one prefix.
type prefixAgg struct {
	prefix string
	asn    uint32
	hosts  uint64
	count  uint64
}

// Extract reads ClickHouse and builds the training data.
func Extract(ctx context.Context, r *reporter.Reporter, conn driver.Conn, c Configuration) (*Result, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}

	result := Result{
		Metadata: flowmodel.Metadata{
			Hyper: c.Model,
		},
	}
	meta := &result.Metadata

	// Build the vocabularies from aggregate queries.
	remoteAggs, localAggs, err := c.topPrefixes(ctx, r, conn)
	if err != nil {
		return nil, err
	}
	// Prefixes seen on the local side belong to the network.
	locals := []netip.Prefix{}
	for _, agg := range localAggs {
		if prefix, err := netip.ParsePrefix(agg.prefix); err == nil && prefix.Bits() > 0 {
			locals = append(locals, prefix)
		}
	}
	remoteTokens := map[string]int32{}
	for _, agg := range remoteAggs {
		prefix, err := netip.ParsePrefix(agg.prefix)
		if err != nil || !c.learnRemote(prefix, agg.asn, locals) {
			continue
		}
		remoteTokens[agg.prefix] = int32(len(meta.RemotePrefixes))
		meta.RemotePrefixes = append(meta.RemotePrefixes, flowmodel.RemotePrefixInfo{
			Prefix: prefix,
			ASN:    agg.asn,
		})
	}
	catchAll4, catchAll6 := netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")
	catchAllRemote := catchAllTokens(len(meta.RemotePrefixes))
	meta.RemotePrefixes = append(meta.RemotePrefixes,
		flowmodel.RemotePrefixInfo{Prefix: catchAll4},
		flowmodel.RemotePrefixInfo{Prefix: catchAll6})
	localTokens := map[string]int32{}
	seenLocal := map[netip.Prefix]bool{}
	overlaps := func(prefix netip.Prefix) bool {
		for seen := range seenLocal {
			if seen.Overlaps(prefix) {
				return true
			}
		}
		return false
	}
	failed := 0
	for _, agg := range localAggs {
		prefix, err := netip.ParsePrefix(agg.prefix)
		if err != nil || prefix.Bits() == 0 {
			continue
		}
		target := c.Anonymization.IPv4Prefix
		if !prefix.Addr().Is4() {
			target = c.Anonymization.IPv6Prefix
		}
		// Without a free slot in the target prefix, the flows use the catch-all
		// entry.
		var anonymized netip.Prefix
		found := false
		for range 100 {
			anonymized, err = flowmodel.AnonymizePrefix(prefix, target)
			if err != nil {
				break
			}
			if found = !overlaps(anonymized); found {
				break
			}
		}
		if !found {
			failed++
			continue
		}
		seenLocal[anonymized] = true
		localTokens[agg.prefix] = int32(len(meta.LocalPrefixes))
		meta.LocalPrefixes = append(meta.LocalPrefixes, flowmodel.LocalPrefixInfo{
			Prefix:   anonymized,
			HostBits: hostBits(agg.hosts),
		})
	}
	if failed > 0 {
		r.Warn().Msgf("cannot anonymize %d local prefixes, their flows use the catch-all entry", failed)
	}
	catchAllLocal := catchAllTokens(len(meta.LocalPrefixes))
	meta.LocalPrefixes = append(meta.LocalPrefixes,
		flowmodel.LocalPrefixInfo{Prefix: catchAll4},
		flowmodel.LocalPrefixInfo{Prefix: catchAll6})
	if len(remoteTokens) == 0 || len(localTokens) == 0 {
		return nil, errors.New("no prefix found in the selected flows")
	}

	protoTokens, protoFallback, err := c.topProtocolsAndPorts(ctx, r, conn, meta)
	if err != nil {
		return nil, err
	}
	portIndex := flowmodel.NewPortIndex(meta.Ports)
	total, err := c.rateProfile(ctx, r, conn, meta)
	if err != nil {
		return nil, err
	}
	r.Info().Msgf("vocabularies: %d remote prefixes, %d local prefixes, %d protocols, %d ports",
		len(meta.RemotePrefixes), len(meta.LocalPrefixes), len(meta.ProtoEtypes), len(meta.Ports))

	// Stream the flows and turn them into weighted examples. When there are too
	// many flows, sample them more to cover the whole time range.
	fraction := c.Fraction
	if total > uint64(c.MaxFlows) {
		// Keep a margin to not reach the limit of the query.
		fraction *= 0.9 * float64(c.MaxFlows) / float64(total)
		r.Info().Msgf("too many flows, keep a fraction of %g for the examples", fraction)
	}
	// The total was counted with the configured fraction.
	expected := uint64(float64(total) * fraction / c.Fraction)
	weights := map[[flowmodel.FieldCount]int32]float32{}
	query := c.streamQuery(fraction).String()
	r.Info().Str("query", "examples").Msg("query started")
	r.Debug().Str("query", "examples").Str("sql", query).Msg("query SQL")
	start := time.Now()
	lastLog := start
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("cannot query flows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			timeReceived         time.Time
			inBText, outBText    string
			etype, proto         uint32
			srcAddr, dstAddr     netip.Addr
			srcPrefix, dstPrefix string
			srcPort, dstPort     uint16
			bytes, packets       uint64
		)
		if err := rows.Scan(&timeReceived, &inBText, &outBText, &etype, &proto,
			&srcAddr, &dstAddr, &srcPrefix, &dstPrefix,
			&srcPort, &dstPort, &bytes, &packets); err != nil {
			return nil, fmt.Errorf("cannot read flow: %w", err)
		}
		result.Flows++
		if now := time.Now(); now.Sub(lastLog) >= 10*time.Second {
			lastLog = now
			r.Info().Dur("duration", now.Sub(start)).
				Msgf("%d flows read, about %d expected", result.Flows, expected)
		}
		var inB, outB schema.InterfaceBoundary
		inB.UnmarshalText([]byte(inBText))
		outB.UnmarshalText([]byte(outBText))

		if packets == 0 {
			continue
		}
		// The query only returns flows with one external interface.
		direction := flowmodel.DirectionInbound
		localPrefix, remotePrefix, localB := dstPrefix, srcPrefix, outB
		if outB == schema.InterfaceBoundaryExternal {
			direction = flowmodel.DirectionOutbound
			localPrefix, remotePrefix, localB = srcPrefix, dstPrefix, inB
		}

		var tokens [flowmodel.FieldCount]int32
		tokens[flowmodel.FieldHourOfWeek] = int32(flowmodel.HourOfWeek(timeReceived))
		tokens[flowmodel.FieldDirection] = int32(direction)
		tokens[flowmodel.FieldLocalBoundary] = int32(localB)
		protoToken, ok := protoTokens[flowmodel.ProtoEtype{EType: uint16(etype), Proto: uint8(proto)}]
		if !ok {
			protoToken, ok = protoFallback[uint16(etype)]
			if !ok {
				continue
			}
		}
		tokens[flowmodel.FieldProtoEtype] = protoToken
		is4 := srcAddr.Unmap().Is4()
		if token, ok := remoteTokens[remotePrefix]; ok {
			tokens[flowmodel.FieldRemotePrefix] = token
		} else {
			tokens[flowmodel.FieldRemotePrefix] = catchAllRemote[is4]
		}
		if token, ok := localTokens[localPrefix]; ok {
			tokens[flowmodel.FieldLocalPrefix] = token
		} else {
			tokens[flowmodel.FieldLocalPrefix] = catchAllLocal[is4]
		}
		tokens[flowmodel.FieldSrcPort] = portIndex.Token(srcPort)
		tokens[flowmodel.FieldDstPort] = portIndex.Token(dstPort)
		tokens[flowmodel.FieldSizeBucket] = flowmodel.SizeBucket(bytes / packets)
		tokens[flowmodel.FieldPacketBucket] = flowmodel.PacketBucket(packets)
		weights[tokens]++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cannot read flows: %w", err)
	}
	if len(weights) == 0 {
		return nil, errors.New("no flow matches the configuration")
	}

	// Sort the examples so the result does not depend on the map
	// iteration order.
	for tokens, weight := range weights {
		result.Examples = append(result.Examples, flowmodel.Example{
			Tokens: tokens,
			Weight: weight,
		})
	}
	slices.SortFunc(result.Examples, func(a, b flowmodel.Example) int {
		return slices.Compare(a.Tokens[:], b.Tokens[:])
	})
	r.Info().Dur("duration", time.Since(start)).
		Msgf("%d flows read, %d distinct examples", result.Flows, len(result.Examples))
	return &result, nil
}

// topPrefixes returns the top remote and local prefixes, merged over both
// directions.
func (c *Configuration) topPrefixes(ctx context.Context, r *reporter.Reporter, conn driver.Conn) ([]prefixAgg, []prefixAgg, error) {
	results := []struct {
		Remote uint8  `ch:"remote"`
		Prefix string `ch:"prefix"`
		ASN    uint32 `ch:"asn"`
		Hosts  uint64 `ch:"hosts"`
		C      uint64 `ch:"c"`
	}{}
	if err := selectWithLog(ctx, r, conn, "prefixes", &results, c.prefixQuery().String()); err != nil {
		return nil, nil, fmt.Errorf("cannot query prefixes: %w", err)
	}
	remote, local := []prefixAgg{}, []prefixAgg{}
	for _, row := range results {
		// Flows that are not IP have an empty prefix.
		if row.C < c.MinCount || row.Prefix == "" {
			continue
		}
		agg := prefixAgg{prefix: row.Prefix, asn: row.ASN, hosts: row.Hosts, count: row.C}
		if row.Remote == 1 {
			remote = append(remote, agg)
		} else {
			local = append(local, agg)
		}
	}
	top := func(aggs []prefixAgg, limit int) []prefixAgg {
		slices.SortFunc(aggs, func(a, b prefixAgg) int {
			return cmp.Or(cmp.Compare(b.count, a.count), cmp.Compare(a.prefix, b.prefix))
		})
		return aggs[:min(len(aggs), limit)]
	}
	return top(remote, c.Model.RemotePrefixes-2), top(local, c.Model.LocalPrefixes-2), nil
}

// topProtocolsAndPorts fills the protocol and port vocabularies. It returns the
// token of each protocol and a fallback token for each ethernet type.
func (c *Configuration) topProtocolsAndPorts(ctx context.Context, r *reporter.Reporter, conn driver.Conn, meta *flowmodel.Metadata) (map[flowmodel.ProtoEtype]int32, map[uint16]int32, error) {
	results := []struct {
		Kind  uint8  `ch:"kind"`
		Value uint32 `ch:"value"`
		C     uint64 `ch:"c"`
	}{}
	query := c.protocolPortQuery().String()
	if err := selectWithLog(ctx, r, conn, "protocols and ports", &results, query); err != nil {
		return nil, nil, fmt.Errorf("cannot query protocols and ports: %w", err)
	}
	protocols := map[flowmodel.ProtoEtype]uint64{}
	ports := map[uint16]uint64{}
	for _, row := range results {
		switch row.Kind {
		case kindProtocol:
			pe := flowmodel.ProtoEtype{EType: uint16(row.Value >> 8), Proto: uint8(row.Value)}
			protocols[pe] += row.C
		case kindSrcPort, kindDstPort:
			ports[uint16(row.Value)] += row.C
		}
	}

	// Protocols, the most common first.
	pes := []flowmodel.ProtoEtype{}
	for pe, count := range protocols {
		if count >= c.MinCount {
			pes = append(pes, pe)
		}
	}
	slices.SortFunc(pes, func(a, b flowmodel.ProtoEtype) int {
		return cmp.Or(cmp.Compare(protocols[b], protocols[a]),
			cmp.Compare(a.EType, b.EType), cmp.Compare(a.Proto, b.Proto))
	})
	pes = pes[:min(len(pes), c.Model.ProtoEtypes)]
	if len(pes) == 0 {
		return nil, nil, errors.New("no protocol found in the selected flows")
	}
	tokens := map[flowmodel.ProtoEtype]int32{}
	fallback := map[uint16]int32{}
	for _, pe := range pes {
		tokens[pe] = int32(len(meta.ProtoEtypes))
		if _, ok := fallback[pe.EType]; !ok {
			// The most common protocol of the family also stands for the rare
			// ones.
			fallback[pe.EType] = tokens[pe]
		}
		meta.ProtoEtypes = append(meta.ProtoEtypes, pe)
	}

	// Ports: the fixed ranges and the most common ports. Port 0 is one of the
	// fixed ranges.
	meta.Ports = flowmodel.DefaultPortRanges()
	top := []uint16{}
	for port, count := range ports {
		if port != 0 && count >= c.MinCount {
			top = append(top, port)
		}
	}
	slices.SortFunc(top, func(a, b uint16) int {
		return cmp.Or(cmp.Compare(ports[b], ports[a]), cmp.Compare(a, b))
	})
	for _, port := range top[:min(len(top), c.Model.Ports-len(meta.Ports))] {
		meta.Ports = append(meta.Ports, flowmodel.PortRange{Start: port, End: port})
	}
	return tokens, fallback, nil
}

// rateProfile fills the hour-of-week rate profile and returns the number of
// flows.
func (c *Configuration) rateProfile(ctx context.Context, r *reporter.Reporter, conn driver.Conn, meta *flowmodel.Metadata) (uint64, error) {
	results := []struct {
		D     uint8     `ch:"d"`
		H     uint8     `ch:"h"`
		C     uint64    `ch:"c"`
		First time.Time `ch:"first"`
		Last  time.Time `ch:"last"`
	}{}
	if err := selectWithLog(ctx, r, conn, "rate profile", &results, c.rateQuery().String()); err != nil {
		return 0, fmt.Errorf("cannot query the rate profile: %w", err)
	}
	if len(results) == 0 {
		return 0, nil
	}
	// Count how many seconds of each hour of the week are read, to
	// convert counts to rates. The time range is reduced to
	// the part with flows, as the database may not cover all of it.
	start, end := results[0].First, results[0].Last
	for _, row := range results {
		if row.First.Before(start) {
			start = row.First
		}
		if row.Last.After(end) {
			end = row.Last
		}
	}
	start, end = start.UTC(), end.UTC().Add(time.Second)
	seconds := [168]float64{}
	for _, bucket := range c.buckets() {
		from, to := bucket, bucket.Add(bucketDuration)
		if from.Before(start) {
			from = start
		}
		if to.After(end) {
			to = end
		}
		if to.After(from) {
			seconds[flowmodel.HourOfWeek(bucket)] += to.Sub(from).Seconds()
		}
	}
	var total uint64
	for _, row := range results {
		// toDayOfWeek gives 1 for Monday and 7 for Sunday. Go uses 0 for
		// Sunday.
		hour := int(row.D%7)*24 + int(row.H)
		if seconds[hour] > 0 {
			meta.RateProfile[hour] = float32(float64(row.C) / seconds[hour])
		}
		total += row.C
	}

	// Hours outside the time range use the same hour of the other days.
	missing := 0
	for hour := range meta.RateProfile {
		if seconds[hour] > 0 {
			continue
		}
		missing++
		var sum float32
		var days int
		for other := hour % 24; other < len(seconds); other += 24 {
			if seconds[other] > 0 {
				sum += meta.RateProfile[other]
				days++
			}
		}
		if days > 0 {
			meta.RateProfile[hour] = sum / float32(days)
		}
	}
	if missing > 0 {
		r.Warn().Msgf("time range does not cover %d hours of the week, use at least a week", missing)
	}

	// Only keep the shape of the profile: the average is 1.
	var sum float64
	for _, rate := range meta.RateProfile {
		sum += float64(rate)
	}
	for hour, rate := range meta.RateProfile {
		meta.RateProfile[hour] = float32(float64(rate) * float64(len(meta.RateProfile)) / sum)
	}
	return total, nil
}

// bogons are the prefixes that are not globally routable.
var bogons = func() []netip.Prefix {
	prefixes := []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
		"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
		"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
		"224.0.0.0/3",
		// Only 2000::/3 is global unicast.
		"::/3", "4000::/2", "8000::/1",
		"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20",
	}
	result := make([]netip.Prefix, len(prefixes))
	for i, prefix := range prefixes {
		result[i] = netip.MustParsePrefix(prefix)
	}
	return result
}()

// learnRemote tells if a remote prefix can enter the model. The prefixes of the
// network, private or very specific prefixes, and private AS numbers would
// identify the network. locals are the prefixes seen on the local side.
func (c *Configuration) learnRemote(prefix netip.Prefix, asn uint32, locals []netip.Prefix) bool {
	maxBits := 48
	if prefix.Addr().Is4() {
		maxBits = 24
	}
	if prefix.Bits() == 0 || prefix.Bits() > maxBits ||
		helpers.IsPrivateAS(asn) || slices.Contains(c.ExcludeASNs, asn) {
		return false
	}
	for _, excluded := range slices.Concat(c.ExcludePrefixes, bogons, locals) {
		if excluded.Overlaps(prefix) {
			return false
		}
	}
	return true
}

// selectWithLog runs a query and logs its duration.
func selectWithLog(ctx context.Context, r *reporter.Reporter, conn driver.Conn, name string, dest any, query string) error {
	r.Info().Str("query", name).Msg("query started")
	r.Debug().Str("query", name).Str("sql", query).Msg("query SQL")
	start := time.Now()
	if err := conn.Select(ctx, dest, query); err != nil {
		return err
	}
	r.Info().Str("query", name).Dur("duration", time.Since(start)).Msg("query finished")
	return nil
}

// catchAllTokens returns the tokens of the IPv4 and IPv6 catch-all entries
// appended to a vocabulary of the given size, indexed by the IPv4 flag.
func catchAllTokens(size int) map[bool]int32 {
	return map[bool]int32{true: int32(size), false: int32(size + 1)}
}

// hostBits returns the log2 of the number of hosts, rounded up, so the model
// does not leak an exact customer count.
func hostBits(hosts uint64) uint8 {
	hosts = min(max(hosts, 16), 65536)
	return uint8(bits.Len64(hosts - 1))
}
