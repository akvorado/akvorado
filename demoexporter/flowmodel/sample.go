// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package flowmodel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"net/netip"
	"slices"
	"time"

	"akvorado/common/constants"
	"akvorado/common/schema"
)

// SamplerOptions alter how flows are generated from the model.
type SamplerOptions struct {
	// Rate is the average number of flows per second. The model only gives the
	// shape of the traffic over the week.
	Rate float64
	// LocalASN is the AS number for the customer side of the flows.
	LocalASN uint32
	// MaxFlowsPerSecond caps the number of generated flows, before the keep
	// callback drops some of them.
	MaxFlowsPerSecond int
}

// Flow is a generated flow.
type Flow struct {
	// Key is a random value to assign the flow to an exporter in a
	// deterministic way.
	Key       uint64
	Direction Direction
	// LocalBoundary is the boundary of the interface on the customer side. The
	// interface on the Internet side is external.
	LocalBoundary schema.InterfaceBoundary
	EType         uint16
	Proto         uint8
	SrcAddr       netip.Addr
	DstAddr       netip.Addr
	SrcMask       uint8
	DstMask       uint8
	SrcPort       uint16
	DstPort       uint16
	SrcAS         uint32
	DstAS         uint32
	RemoteASN     uint32 // RemoteASN is the AS number of the Internet side of the flow.
	Tag           int
	Octets        uint64
	Packets       uint32
}

// Sampler generates flows from a model. It is not safe for
// concurrent use.
type Sampler struct {
	m    *Model
	opts SamplerOptions

	// Buffers reused between flows.
	acc, u, h, logits []float32
	tokens            [FieldCount]int32

	remote, local familyIndex
	// protoMask excludes the protocols of an address family without
	// a concrete remote or local prefix.
	protoMask []bool
}

// addressFamily is an IP address family.
type addressFamily int

const (
	familyIPv6 addressFamily = iota
	familyIPv4
	familyCount
)

// familyIndex splits a prefix vocabulary by address family.
type familyIndex struct {
	// concrete lists the tokens of the prefixes that are not catch-all.
	concrete [familyCount][]int32
	// mask tells for each token if it belongs to the family.
	mask [familyCount][]bool
}

// family returns the address family of a prefix.
func family(prefix netip.Prefix) addressFamily {
	if prefix.Addr().Is4() {
		return familyIPv4
	}
	return familyIPv6
}

// etypeFamily returns the address family of an ethernet type.
func etypeFamily(etype uint16) addressFamily {
	if etype == constants.ETypeIPv4 {
		return familyIPv4
	}
	return familyIPv6
}

// add appends the next prefix of the vocabulary to the index.
func (fi *familyIndex) add(prefix netip.Prefix) {
	fam := family(prefix)
	if prefix.Bits() > 0 {
		fi.concrete[fam] = append(fi.concrete[fam], int32(len(fi.mask[fam])))
	}
	for f := range familyCount {
		fi.mask[f] = append(fi.mask[f], f == fam)
	}
}

// resolve replaces a catch-all token with a random concrete token of the same
// family. A catch-all entry stands for the long tail of prefixes.
func (fi *familyIndex) resolve(token int32, prefix netip.Prefix, r *rand.Rand) int32 {
	if prefix.Bits() > 0 {
		return token
	}
	pool := fi.concrete[family(prefix)]
	return pool[r.IntN(len(pool))]
}

// NewSampler creates a sampler for the model.
func (m *Model) NewSampler(opts SamplerOptions) (*Sampler, error) {
	sizes := m.Metadata.vocabSizes()
	dH := m.Metadata.Hyper.HiddenDim
	s := Sampler{
		m:      m,
		opts:   opts,
		acc:    make([]float32, dH),
		u:      make([]float32, dH),
		h:      make([]float32, dH),
		logits: make([]float32, slices.Max(sizes[:])),
	}
	for _, info := range m.Metadata.RemotePrefixes {
		s.remote.add(info.Prefix)
	}
	for _, info := range m.Metadata.LocalPrefixes {
		s.local.add(info.Prefix)
	}
	usable := false
	for _, protoEtype := range m.Metadata.ProtoEtypes {
		fam := etypeFamily(protoEtype.EType)
		ok := len(s.remote.concrete[fam]) > 0 && len(s.local.concrete[fam]) > 0
		s.protoMask = append(s.protoMask, ok)
		usable = usable || ok
	}
	if !usable {
		return nil, errors.New("model has no usable prefix")
	}
	return &s, nil
}

// Generate returns one second worth of flows for the given time. The
// result only depends on the model, the options, the seed and the
// time, so several instances using the same inputs produce the same
// flows. When keep is not nil, it is called early with a flow only
// holding Key, Direction and RemoteASN. The flow is dropped when it
// returns false.
func (s *Sampler) Generate(seed int64, now time.Time, keep func(*Flow) bool) []Flow {
	now = now.Truncate(time.Second)

	// Initialize the random number generator to a known state.
	hash := fnv.New64()
	fmt.Fprintf(hash, "%d %d", now.Unix(), seed)
	r := rand.New(rand.NewPCG(hash.Sum64(), 0))

	how, other, weight := hourMix(now)
	profile := &s.m.Metadata.RateProfile
	rate := ((1-weight)*float64(profile[how]) + weight*float64(profile[other])) *
		s.opts.Rate * (0.9 + r.Float64()/5)
	count := min(rateToCount(rate, now), s.opts.MaxFlowsPerSecond)
	flows := make([]Flow, 0, count)
	// Each flow has its own generator, so a dropped flow does not change the
	// next ones.
	flowSource := rand.NewPCG(0, 0)
	flowRand := rand.New(flowSource)
	for range count {
		flowSource.Seed(r.Uint64(), r.Uint64())
		// The traffic mix also moves smoothly from one hour to the next.
		hour := how
		if flowRand.Float64() < weight {
			hour = other
		}
		if flow, ok := s.sampleFlow(int32(hour), flowRand, keep); ok {
			flows = append(flows, flow)
		}
	}
	return flows
}

// hourMix returns the hour of the week of a time, the nearest other hour, and
// the weight of the other hour. An hourly value is for the middle of the hour,
// so the values of two hours are mixed to avoid steps.
func hourMix(now time.Time) (how, other int, weight float64) {
	how = HourOfWeek(now)
	now = now.UTC()
	offset := float64(now.Minute()*60+now.Second())/3600 - 0.5
	other = (how + 1) % hourOfWeekCount
	if offset < 0 {
		other = (how + hourOfWeekCount - 1) % hourOfWeekCount
	}
	return how, other, math.Abs(offset)
}

// sampleFlow samples the tokens of one flow and turns them into a flow.
func (s *Sampler) sampleFlow(hourOfWeek int32, r *rand.Rand, keep func(*Flow) bool) (Flow, bool) {
	meta := &s.m.Metadata
	tokens := &s.tokens
	flow := Flow{Key: r.Uint64()}
	var remote RemotePrefixInfo
	copy(s.acc, s.m.w.b1)
	tokens[FieldHourOfWeek] = hourOfWeek
	s.m.contextAdd(s.acc, FieldHourOfWeek, hourOfWeek)
	for f := 1; f < FieldCount; f++ {
		var mask []bool
		switch f {
		case FieldProtoEtype:
			mask = s.protoMask
		case FieldRemotePrefix:
			mask = s.remote.mask[etypeFamily(meta.ProtoEtypes[tokens[FieldProtoEtype]].EType)]
		case FieldLocalPrefix:
			mask = s.local.mask[etypeFamily(meta.ProtoEtypes[tokens[FieldProtoEtype]].EType)]
		}
		token := s.sampleField(f, mask, r)
		tokens[f] = token
		if f < FieldCount-1 {
			s.m.contextAdd(s.acc, f, token)
		}
		if f == FieldRemotePrefix {
			// This is enough to know which exporter gets the flow.
			remote = meta.RemotePrefixes[s.remote.resolve(token, meta.RemotePrefixes[token].Prefix, r)]
			flow.Direction = Direction(tokens[FieldDirection])
			flow.RemoteASN = remote.ASN
			if keep != nil && !keep(&flow) {
				return Flow{}, false
			}
		}
	}

	protoEtype := meta.ProtoEtypes[tokens[FieldProtoEtype]]
	flow.LocalBoundary = schema.InterfaceBoundary(tokens[FieldLocalBoundary])
	flow.EType, flow.Proto = protoEtype.EType, protoEtype.Proto

	local := meta.LocalPrefixes[s.local.resolve(tokens[FieldLocalPrefix],
		meta.LocalPrefixes[tokens[FieldLocalPrefix]].Prefix, r)]
	remoteAddr := hostIP(remote.Prefix, r.Uint64(), r.Uint64())
	hi, lo := r.Uint64(), r.Uint64()
	if local.HostBits > 0 {
		// The same rank always gives the same address.
		hash := fnv.New64a()
		fmt.Fprintf(hash, "%s %d", local.Prefix, zipfRank(r, uint32(1)<<local.HostBits))
		hi = hash.Sum64()
		hash.Write([]byte{0})
		lo = hash.Sum64()
	}
	localAddr := hostIP(local.Prefix, hi, lo)
	if flow.Direction == DirectionInbound {
		flow.SrcAddr, flow.DstAddr = remoteAddr, localAddr
		flow.SrcMask, flow.DstMask = uint8(remote.Prefix.Bits()), uint8(local.Prefix.Bits())
		flow.SrcAS, flow.DstAS = remote.ASN, s.opts.LocalASN
	} else {
		flow.SrcAddr, flow.DstAddr = localAddr, remoteAddr
		flow.SrcMask, flow.DstMask = uint8(local.Prefix.Bits()), uint8(remote.Prefix.Bits())
		flow.SrcAS, flow.DstAS = s.opts.LocalASN, remote.ASN
	}

	if flow.Proto == constants.ProtoTCP || flow.Proto == constants.ProtoUDP {
		flow.SrcPort = portValue(meta.Ports[tokens[FieldSrcPort]], r)
		flow.DstPort = portValue(meta.Ports[tokens[FieldDstPort]], r)
	}

	packetLo, packetHi := packetBucketRange(tokens[FieldPacketBucket])
	sizeLo, sizeHi := sizeBucketRange(tokens[FieldSizeBucket])
	sizeLo = max(sizeLo, 1)
	packets := packetLo + r.Uint64N(packetHi-packetLo+1)
	flow.Packets = uint32(packets)
	flow.Octets = packets * (sizeLo + r.Uint64N(sizeHi-sizeLo+1))
	return flow, true
}

// sampleField samples a token for a field from the current context. Tokens
// excluded by the mask are never picked. The mask must keep at least one token.
func (s *Sampler) sampleField(field int, mask []bool, r *rand.Rand) int32 {
	vocab := s.m.Metadata.vocabSizes()[field]
	logits := s.logits[:vocab]
	s.m.forwardStep(s.acc, field, s.u, s.h, logits)

	maxLogit := float32(math.Inf(-1))
	for i, l := range logits {
		if mask == nil || mask[i] {
			maxLogit = max(maxLogit, l)
		}
	}
	var sum float64
	for i, l := range logits {
		logits[i] = 0
		if mask == nil || mask[i] {
			logits[i] = float32(math.Exp(float64(l - maxLogit)))
		}
		sum += float64(logits[i])
	}
	// Excluded tokens have a zero weight. They are skipped, even when the
	// target is zero.
	target := r.Float64() * sum
	last := int32(-1)
	var acc float64
	for i, l := range logits {
		if l == 0 {
			continue
		}
		last = int32(i)
		acc += float64(l)
		if acc >= target {
			break
		}
	}
	return last
}

// portValue turns a port vocabulary entry into a port number.
func portValue(p PortRange, r *rand.Rand) uint16 {
	if p.Start == p.End {
		return p.Start
	}
	return p.Start + uint16(r.IntN(int(p.End-p.Start)+1))
}

// zipfRank picks a rank in [0, pool) with a bias towards low ranks, so a few
// hosts get most of the traffic.
func zipfRank(r *rand.Rand, pool uint32) uint64 {
	rank := uint64(math.Exp(r.Float64()*math.Log(float64(pool)))) - 1
	if rank >= uint64(pool) {
		rank = uint64(pool) - 1
	}
	return rank
}

// hostIP returns the address of a prefix whose host bits come from hi and lo.
func hostIP(prefix netip.Prefix, hi, lo uint64) netip.Addr {
	var host [16]byte
	binary.BigEndian.PutUint64(host[:8], hi)
	binary.BigEndian.PutUint64(host[8:], lo)
	base := prefix.Masked().Addr().AsSlice()
	for i := prefix.Bits(); i < len(base)*8; i++ {
		setBit(base, i, getBit(host[:], i))
	}
	addr, _ := netip.AddrFromSlice(base)
	return addr
}

// rateToCount converts a per-second rate to the number of items to produce for
// the given time.
func rateToCount(rate float64, now time.Time) int {
	seconds := float64(now.Unix() - now.Truncate(time.Hour*24*30*12).Unix())
	count := math.Trunc((seconds+1)*rate) - math.Trunc(seconds*rate)
	return int(count)
}
