// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
)

// edgeFilter builds the condition keeping the flows crossing the edge of the
// network: inbound flows enter on an external interface and leave on a
// non-external one, outbound flows do the opposite.
func edgeFilter() sb.Expr {
	external := sb.String(schema.InterfaceBoundaryExternal.String())
	inbound := sb.And(
		sb.Op(sb.Column("InIfBoundary"), "=", external),
		sb.Op(sb.Column("OutIfBoundary"), "!=", external))
	outbound := sb.And(
		sb.Op(sb.Column("OutIfBoundary"), "=", external),
		sb.Op(sb.Column("InIfBoundary"), "!=", external))
	return sb.Parens(sb.Or(sb.Parens(inbound), sb.Parens(outbound)))
}

// remoteOrLocal returns the remote column of a pair for remote, the local one
// otherwise. The source is remote for inbound flows.
func remoteOrLocal(remote bool, src, dst string) sb.Expr {
	inbound := sb.Op(sb.Column("InIfBoundary"), "=",
		sb.String(schema.InterfaceBoundaryExternal.String()))
	if !remote {
		src, dst = dst, src
	}
	return sb.Function("if", inbound, sb.Column(src), sb.Column(dst))
}

// sampleFilter builds the condition keeping a fraction of the flows. Use a hash
// of the flow.
func sampleFilter(fraction float64) sb.Expr {
	// Small columns only: they are read for every row.
	hash := sb.Function("murmurHash3_32", sb.Columns(
		"TimeReceived", "SrcPort", "DstPort")...)
	return sb.Op(hash, "<", sb.Uint(uint64(fraction*(1<<32))))
}

// bucketDuration is the duration of the buckets of the primary key of the flows
// table.
const bucketDuration = 5 * time.Minute

// bucketsPerHour returns how many buckets of each hour are read. It is the
// lowest number that keeps the configured fraction of the flows.
func (c *Configuration) bucketsPerHour() int {
	const total = int(time.Hour / bucketDuration)
	return min(total, int(math.Ceil(c.Fraction*float64(total))))
}

// buckets returns the start of the buckets read, in order.
func (c *Configuration) buckets() []time.Time {
	const total = int(time.Hour / bucketDuration)
	perHour := c.bucketsPerHour()
	buckets := []time.Time{}
	for hour := c.Start.UTC().Truncate(time.Hour); hour.Before(c.End); hour = hour.Add(time.Hour) {
		// The choice only depends on the hour, so all the queries and all
		// the time ranges get the same buckets.
		rnd := rand.New(rand.NewPCG(uint64(hour.Unix()), 0))
		for _, i := range rnd.Perm(total)[:perHour] {
			bucket := hour.Add(time.Duration(i) * bucketDuration)
			if bucket.Add(bucketDuration).After(c.Start) && bucket.Before(c.End) {
				buckets = append(buckets, bucket)
			}
		}
	}
	slices.SortFunc(buckets, time.Time.Compare)
	return buckets
}

// commonFilter builds the WHERE clause shared by all the queries. The sampling
// is part of every query, so even the aggregates only see a fraction of the
// flows.
func (c *Configuration) commonFilter(fraction float64) sb.Expr {
	const timeFormat = "2006-01-02 15:04:05"
	toDateTime := func(t time.Time) sb.Expr {
		return sb.Function("toDateTime", sb.String(t.UTC().Format(timeFormat)), sb.String("UTC"))
	}
	conditions := []sb.Expr{
		sb.Op(sb.Column("TimeReceived"), ">=", toDateTime(c.Start)),
		sb.Op(sb.Column("TimeReceived"), "<", toDateTime(c.End)),
	}
	// Selecting buckets of the primary key skips the other ones without
	// reading them. The hash keeps the remaining fraction inside them.
	if perHour := c.bucketsPerHour(); perHour < int(time.Hour/bucketDuration) {
		buckets := []sb.Expr{}
		for _, bucket := range c.buckets() {
			buckets = append(buckets, toDateTime(bucket))
		}
		conditions = append(conditions, sb.Op(
			sb.Function("toStartOfFiveMinutes", sb.Column("TimeReceived")),
			"IN", sb.Tuple(buckets...)))
		fraction *= float64(time.Hour/bucketDuration) / float64(perHour)
	}
	if fraction < 1 {
		conditions = append(conditions, sampleFilter(fraction))
	}
	if len(c.Exporters) > 0 {
		names := make([]sb.Expr, len(c.Exporters))
		for i, name := range c.Exporters {
			names[i] = sb.String(name)
		}
		conditions = append(conditions,
			sb.Op(sb.Column("ExporterName"), "IN", sb.Tuple(names...)))
	}
	conditions = append(conditions, edgeFilter())
	return sb.And(conditions...)
}

// prefixQuery builds a query returning the prefixes of both sides of the flows,
// with their most common AS number, the approximate number of distinct hosts,
// and the number of flows. Each flow gives one row for each side. The AS number
// is only read for the remote side.
func (c *Configuration) prefixQuery() *sb.Query {
	sides := sb.Array(
		sb.Tuple(sb.Uint(1),
			remoteOrLocal(true, "SrcNetPrefix", "DstNetPrefix"),
			remoteOrLocal(true, "SrcAS", "DstAS"),
			remoteOrLocal(true, "SrcAddr", "DstAddr")),
		sb.Tuple(sb.Uint(0),
			remoteOrLocal(false, "SrcNetPrefix", "DstNetPrefix"),
			sb.Cast(sb.Uint(0), "UInt32"),
			remoteOrLocal(false, "SrcAddr", "DstAddr")))
	side := func(index int64) sb.Expr {
		return sb.Function("tupleElement", sb.Column("side"), sb.Int(index))
	}
	return sb.Select().
		Item(sb.Alias(side(1), "remote")).
		Item(sb.Alias(side(2), "prefix")).
		Item(sb.Alias(sb.Function("anyHeavy", side(3)), "asn")).
		Item(sb.Alias(sb.Function("uniq", side(4)), "hosts")).
		Item(sb.Alias(sb.Function("count"), "c")).
		From(sb.Table("flows")).
		ArrayJoin(sb.Alias(sides, "side")).
		Where(c.commonFilter(c.Fraction)).
		GroupBy(sb.Column("remote"), sb.Column("prefix"))
}

const (
	kindProtocol uint8 = iota
	kindSrcPort
	kindDstPort
)

// protocolPortQuery builds a query returning the number of flows for each
// protocol and ethernet type pair, and for each source and destination port of
// TCP and UDP flows. Each flow gives one row for its protocol, encoded as
// EType×256+Proto, and one row for each of its ports.
func (c *Configuration) protocolPortQuery() *sb.Query {
	item := func(kind uint8, value sb.Expr) sb.Expr {
		return sb.Tuple(sb.Uint(uint64(kind)), sb.Function("toUInt32", value))
	}
	protocol := item(kindProtocol, sb.Function("plus",
		sb.Function("multiply", sb.Column("EType"), sb.Int(256)), sb.Column("Proto")))
	items := sb.Function("if",
		sb.Op(sb.Column("Proto"), "IN", sb.Tuple(sb.Int(6), sb.Int(17))),
		sb.Array(protocol,
			item(kindSrcPort, sb.Column("SrcPort")),
			item(kindDstPort, sb.Column("DstPort"))),
		sb.Array(protocol))
	return sb.Select().
		Item(sb.Alias(sb.Function("tupleElement", sb.Column("item"), sb.Int(1)), "kind")).
		Item(sb.Alias(sb.Function("tupleElement", sb.Column("item"), sb.Int(2)), "value")).
		Item(sb.Alias(sb.Function("count"), "c")).
		From(sb.Table("flows")).
		ArrayJoin(sb.Alias(items, "item")).
		Where(c.commonFilter(c.Fraction)).
		GroupBy(sb.Column("kind"), sb.Column("value"))
}

// rateQuery builds a query returning the number of flows for each hour of the
// week, in UTC, with the time of the first and last flows.
func (c *Configuration) rateQuery() *sb.Query {
	return sb.Select().
		Item(sb.Alias(sb.Function("toDayOfWeek",
			sb.Column("TimeReceived"), sb.Int(0), sb.String("UTC")), "d")).
		Item(sb.Alias(sb.Function("toHour", sb.Column("TimeReceived"), sb.String("UTC")), "h")).
		Item(sb.Alias(sb.Function("count"), "c")).
		Item(sb.Alias(sb.Function("min", sb.Column("TimeReceived")), "first")).
		Item(sb.Alias(sb.Function("max", sb.Column("TimeReceived")), "last")).
		From(sb.Table("flows")).
		Where(c.commonFilter(c.Fraction)).
		GroupBy(sb.Column("d"), sb.Column("h"))
}

// streamQuery builds the query returning the flows used to build the training
// examples. It only reads columns the model is allowed to learn. The fraction
// should be low enough to not reach the limit, so the flows are spread over the
// whole time range.
func (c *Configuration) streamQuery(fraction float64) *sb.Query {
	return sb.Select(sb.Columns(
		"TimeReceived",
		"InIfBoundary", "OutIfBoundary",
		"EType", "Proto",
		"SrcAddr", "DstAddr",
		"SrcNetPrefix", "DstNetPrefix",
		"SrcPort", "DstPort",
		"Bytes", "Packets")...).
		From(sb.Table("flows")).
		Where(c.commonFilter(fraction)).
		Limit(c.MaxFlows)
}
