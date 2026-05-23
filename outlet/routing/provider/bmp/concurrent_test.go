// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package bmp

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"sync"
	"testing"
	"time"

	"akvorado/common/helpers"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

func BenchmarkRIBConcurrent(b *testing.B) {
	for _, shards := range []int{1, 16} {
		for _, routes := range []int{10_000, 100_000, 500_000} {
			for _, writers := range []int{0, 1, 2, 4, 8} {
				for _, readers := range []int{1, 4, 16, 32} {
					name := fmt.Sprintf("%d shards, %d routes, %d writers, %d readers", shards, routes, writers, readers)
					b.Run(name, func(b *testing.B) {
						// Pre-generate a pool of routes per writer. With 0
						// writers, still generate one writer's worth to
						// pre-populate the RIB. The pool is 25% larger than the
						// initial live set so writers always have unused
						// entries available for announcements.
						numWriters := max(writers, 1)
						routesPerWriter := routes / numWriters
						poolSize := routesPerWriter * 5 / 4
						writerRoutes := make([][]randomRoute, numWriters)
						// Scratch slice per writer that partitions pool indices
						// into live (left) and dead (right) halves.
						writerScratch := make([][]int, numWriters)
						for w := range numWriters {
							prng1 := rand.New(rand.NewPCG(uint64(w*2+100), uint64(w*2+100)))
							prng2 := rand.New(rand.NewPCG(uint64(w*2+101), uint64(w*2+101)))
							writerRoutes[w] = make([]randomRoute, 0, poolSize)
							for r := range randomRealWorldRoutes4(prng1, prng2, poolSize) {
								writerRoutes[w] = append(writerRoutes[w], r)
							}
							writerScratch[w] = make([]int, poolSize)
						}

						addOne := func(rib *rib, w int, r randomRoute) {
							// nh is the IPv4-mapped IPv6 next hop ::ffff:198.51.100.<w+1>.
							nh := netip.AddrFrom16([16]byte{10: 0xff, 11: 0xff, 12: 198, 13: 51, 14: 100, 15: byte(w + 1)})
							pfx := helpers.PrefixTo6(r.Prefix)
							rib.AddRoute(pfx, rawRoute{
								peer:    uint32(w),
								nlri:    nlri{family: bgp.RF_IPv4_UC},
								nextHop: nextHop(nh),
								attributes: routeAttributes{
									asn:              r.ASPath[len(r.ASPath)-1],
									asPath:           r.ASPath,
									communities:      r.Communities,
									largeCommunities: r.LargeCommunities,
								},
								prefixLen: uint8(pfx.Bits()),
							})
						}
						removeOne := func(rib *rib, w int, r randomRoute) {
							pfx := helpers.PrefixTo6(r.Prefix)
							rib.RemoveRoute(pfx, rawRoute{
								peer: uint32(w),
								nlri: nlri{family: bgp.RF_IPv4_UC},
							})
						}

						// prePopulate seeds the rib with the writer's initial
						// live set.
						prePopulate := func(rib *rib, w int) {
							for i := range routesPerWriter {
								addOne(rib, w, writerRoutes[w][i])
							}
						}

						// runWriter performs routesPerWriter operations on rib
						// using the following churn mix: 20% withdrawals, 20%
						// new announcements, 60% attribute updates of live
						// prefixes.
						//
						// The pre-allocated writerScratch[w] slice is used as a
						// partition: indices [0:nLive) are live and [nLive:)
						// are dead.
						runWriter := func(rib *rib, w int) int64 {
							prng := rand.New(rand.NewPCG(uint64(w+1000), uint64(w+1000)))
							pool := writerRoutes[w]
							live := writerScratch[w]
							for i := range live {
								live[i] = i
							}
							nLive := routesPerWriter
							var count int64
							for range routesPerWriter {
								roll := prng.IntN(10)
								switch {
								case roll < 2 && nLive > 0:
									// Withdraw a random live prefix.
									idx := prng.IntN(nLive)
									j := live[idx]
									removeOne(rib, w, pool[j])
									live[idx] = live[nLive-1]
									live[nLive-1] = j
									nLive--
								case roll < 4 && nLive < len(live):
									// Announce a previously-dead prefix.
									addOne(rib, w, pool[live[nLive]])
									nLive++
								case nLive > 0:
									// Update the attributes of a random live prefix.
									addOne(rib, w, pool[live[prng.IntN(nLive)]])
								case nLive < len(live):
									// Live set exhausted: forced announce.
									addOne(rib, w, pool[live[nLive]])
									nLive++
								default:
									continue
								}
								count++
							}
							return count
						}

						// Pre-generate lookup targets for readers
						prngLookup1 := rand.New(rand.NewPCG(999, 999))
						prngLookup2 := rand.New(rand.NewPCG(1000, 1000))
						lookupTargets := make([]randomRoute, 0, 10_000)
						for r := range randomRealWorldRoutes4(prngLookup1, prngLookup2, 10_000) {
							lookupTargets = append(lookupTargets, r)
						}

						// With 0 writers, pre-populate a single read-only RIB.
						var prepopulatedRIB *rib
						if writers == 0 {
							prepopulatedRIB = newRIB(shards)
							prePopulate(prepopulatedRIB, 0)
						}

						// Accumulated per-goroutine timing across all b.Loop() iterations
						writerTimes := make([]time.Duration, writers)
						writerCounts := make([]int64, writers)
						readerTimes := make([]time.Duration, readers)
						readerCounts := make([]int64, readers)

						for b.Loop() {
							rib := prepopulatedRIB
							if writers > 0 {
								rib = newRIB(shards)
								var prepWg sync.WaitGroup
								for w := range writers {
									prepWg.Go(func() {
										prePopulate(rib, w)
									})
								}
								prepWg.Wait()
							}
							var writerWg sync.WaitGroup
							readerDone := make(chan struct{})

							// Start writers
							for w := range writers {
								writerWg.Go(func() {
									start := time.Now()
									count := runWriter(rib, w)
									writerTimes[w] += time.Since(start)
									writerCounts[w] += count
								})
							}

							// Start readers concurrently with writers
							var readerWg sync.WaitGroup
							for rd := range readers {
								readerWg.Go(func() {
									var count int64
									offset := rd * len(lookupTargets) / readers
									start := time.Now()
								reading:
									for {
										if writers == 0 {
											// No writers: perform a fixed number of lookups
											// against the pre-populated RIB.
											if count >= int64(len(lookupTargets)) {
												break
											}
										} else {
											select {
											case <-readerDone:
												break reading
											default:
											}
										}
										idx := offset % len(lookupTargets)
										ip := lookupTargets[idx].Prefix.Addr()
										rib.LookupRoute(ip, netip.Addr{})
										count++
										offset++
									}
									readerTimes[rd] += time.Since(start)
									readerCounts[rd] += count
								})
							}

							writerWg.Wait()
							close(readerDone)
							readerWg.Wait()
						}

						// Aggregate: average ns/op across goroutines and iterations
						var totalWriteNs, totalWriteOps int64
						for w := range writers {
							totalWriteNs += writerTimes[w].Nanoseconds()
							totalWriteOps += writerCounts[w]
						}
						var totalReadNs, totalReadOps int64
						for rd := range readers {
							totalReadNs += readerTimes[rd].Nanoseconds()
							totalReadOps += readerCounts[rd]
						}

						b.ReportMetric(0, "ns/op")
						if writers > 0 {
							b.ReportMetric(float64(totalWriteNs)/float64(totalWriteOps), "ns/write")
						}
						b.ReportMetric(float64(totalReadNs)/float64(totalReadOps), "ns/read")
					})
				}
			}
		}
	}
}
