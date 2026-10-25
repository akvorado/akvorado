// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/httpserver"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/console/authentication"
	"akvorado/console/database"
)

func TestFilterCompleteClickHouse(t *testing.T) {
	r := reporter.NewMock(t)
	chdb := clickhousedb.SetupClickHouse(t, r, false)
	config := schema.DefaultConfiguration()
	config.Enabled = []schema.ColumnKey{
		schema.ColumnICMPv6, schema.ColumnICMPv6Type, schema.ColumnICMPv6Code,
		schema.ColumnSrcMAC, schema.ColumnDstMAC,
	}
	config.CustomDictionaries = map[string]schema.CustomDict{
		"test": {
			Keys:       []schema.CustomDictKey{{Name: "addr", Type: "String"}},
			Attributes: []schema.CustomDictAttribute{{Name: "role", Type: "String"}},
			Source:     "test.csv",
			Layout:     "hashed",
			Dimensions: []string{"DstAddr"},
		},
	}
	sch, err := schema.New(config)
	if err != nil {
		t.Fatalf("schema.New() error:\n%+v", err)
	}

	ctx := clickhouse.Context(t.Context(), clickhouse.WithSettings(clickhouse.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))
	dbName := chdb.DatabaseName()
	statements := []string{
		// Dictionaries, loaded from plain tables
		"CREATE TABLE asns_source (asn UInt32, name String) ENGINE = Memory",
		`INSERT INTO asns_source VALUES
 (1299, 'Telia'), (15169, 'Google'), (16550, 'Google Private Cloud'),
 (19527, 'Google'), (36040, 'Google'), (200000, 'Not Google'), (396982, 'Google Cloud')`,
		fmt.Sprintf(`CREATE DICTIONARY asns (asn UInt32, name String)
PRIMARY KEY asn SOURCE(CLICKHOUSE(DB '%s' TABLE 'asns_source'))
LIFETIME(0) LAYOUT(HASHED())`, dbName),
		"CREATE TABLE tcp_source (port UInt16, name String) ENGINE = Memory",
		"INSERT INTO tcp_source VALUES (80, 'http'), (443, 'https'), (8080, 'http-alt')",
		fmt.Sprintf(`CREATE DICTIONARY tcp (port UInt16, name String)
PRIMARY KEY port SOURCE(CLICKHOUSE(DB '%s' TABLE 'tcp_source'))
LIFETIME(0) LAYOUT(HASHED())`, dbName),
		"CREATE TABLE udp_source (port UInt16, name String) ENGINE = Memory",
		"INSERT INTO udp_source VALUES (53, 'domain'), (443, 'https')",
		fmt.Sprintf(`CREATE DICTIONARY udp (port UInt16, name String)
PRIMARY KEY port SOURCE(CLICKHOUSE(DB '%s' TABLE 'udp_source'))
LIFETIME(0) LAYOUT(HASHED())`, dbName),
		"CREATE TABLE icmp_source (proto UInt8, type UInt8, code UInt8, name String) ENGINE = Memory",
		`INSERT INTO icmp_source VALUES
 (1, 0, 0, 'echo-reply'), (1, 8, 0, 'echo-request'),
 (58, 1, 0, 'destination-unreachable'), (58, 128, 0, 'echo-request'), (58, 129, 0, 'echo-reply'),
 (58, 160, 0, 'extended-echo-request'), (58, 161, 0, 'extended-echo-reply')`,
		fmt.Sprintf(`CREATE DICTIONARY icmp (proto UInt8, type UInt8, code UInt8, name String)
PRIMARY KEY proto, type, code SOURCE(CLICKHOUSE(DB '%s' TABLE 'icmp_source'))
LIFETIME(0) LAYOUT(COMPLEX_KEY_HASHED())`, dbName),

		fmt.Sprintf("CREATE TABLE flows (%s) ENGINE = MergeTree ORDER BY TimeReceived",
			sch.ClickHouseCreateTable()),
		"CREATE TABLE exporters (ExporterName LowCardinality(String), IfName LowCardinality(String)) ENGINE = Memory",
		`INSERT INTO exporters VALUES
 ('th2-router2', 'Gi0/1'), ('th2-router1', 'Gi0/0'), ('th2-router1', 'Te0/0'),
 ('edge-th2-1', 'gi0/2'), ('par-router1', 'Te0/1')`,

		// Flows. The most used values do not have the lowest numbers, so the
		// order of the results tells if the counts are used or not.
		"INSERT INTO flows (TimeReceived, SrcMAC) SELECT now(), 0x113333445566 FROM numbers(5)",
		"INSERT INTO flows (TimeReceived, SrcMAC) SELECT now(), 0x112233445566 FROM numbers(3)",
		"INSERT INTO flows (TimeReceived, SrcMAC) SELECT now(), 0x11ff33445566 FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, SrcMAC) SELECT now(), 0x223344556677 FROM numbers(10)",
		"INSERT INTO flows (TimeReceived, SrcNetName) SELECT now(), 'my-customer' FROM numbers(5)",
		"INSERT INTO flows (TimeReceived, SrcNetName) SELECT now(), 'customer-b' FROM numbers(3)",
		"INSERT INTO flows (TimeReceived, SrcNetName) SELECT now(), 'customer-a' FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, SrcNetName) SELECT now(), 'other' FROM numbers(10)",
		"INSERT INTO flows (TimeReceived, DstAddrRole) SELECT now(), 'a-role' FROM numbers(5)",
		"INSERT INTO flows (TimeReceived, DstAddrRole) SELECT now(), 'b-role' FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, DstAddrRole) SELECT now(), 'role-x' FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, DstAddrRole) SELECT now(), 'other' FROM numbers(3)",
		"INSERT INTO flows (TimeReceived, DstAS) SELECT now(), 396982 FROM numbers(5)",
		"INSERT INTO flows (TimeReceived, DstAS) SELECT now(), 19527 FROM numbers(3)",
		"INSERT INTO flows (TimeReceived, DstAS) SELECT now(), 36040 FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, DstAS) SELECT now(), 15169 FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, DstAS) SELECT now(), 1299 FROM numbers(10)",
		"INSERT INTO flows (TimeReceived, Proto, DstPort) SELECT now(), 6, 8080 FROM numbers(5)",
		"INSERT INTO flows (TimeReceived, Proto, DstPort) SELECT now(), 6, 443 FROM numbers(3)",
		"INSERT INTO flows (TimeReceived, Proto, DstPort) SELECT now(), 17, 53 FROM numbers(10)",
		"INSERT INTO flows (TimeReceived, Proto, ICMPv6Type, ICMPv6Code) SELECT now(), 58, 160, 0 FROM numbers(5)",
		"INSERT INTO flows (TimeReceived, Proto, ICMPv6Type, ICMPv6Code) SELECT now(), 58, 129, 0 FROM numbers(3)",
		"INSERT INTO flows (TimeReceived, Proto, ICMPv6Type, ICMPv6Code) SELECT now(), 58, 128, 0 FROM numbers(1)",
		"INSERT INTO flows (TimeReceived, Proto, ICMPv6Type, ICMPv6Code) SELECT now(), 58, 1, 0 FROM numbers(10)",
		`INSERT INTO flows (TimeReceived, DstCommunities)
 SELECT now(), [toUInt32(65401 * 65536 + 12)] FROM numbers(3)`,
		`INSERT INTO flows (TimeReceived, DstCommunities)
 SELECT now(), [toUInt32(65401 * 65536 + 10)] FROM numbers(1)`,
		`INSERT INTO flows (TimeReceived, DstLargeCommunities)
 SELECT now(), [bitShiftLeft(toUInt128(65401), 64) + bitShiftLeft(toUInt128(100), 32) + 200] FROM numbers(2)`,
	}
	for _, statement := range statements {
		if err := chdb.Exec(ctx, statement); err != nil {
			t.Fatalf("Exec(%q) error:\n%+v", statement, err)
		}
	}

	h := httpserver.NewMock(t, r)
	c, err := New(r, DefaultConfiguration(), Dependencies{
		Daemon:       daemon.NewMock(t),
		HTTP:         h,
		ClickHouseDB: chdb,
		Auth:         authentication.NewMock(t, r),
		Database:     database.NewMock(t, r, database.DefaultConfiguration()),
		Schema:       sch,
	})
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
	helpers.StartStop(t, c)

	helpers.TestHTTPEndpoints(t, h.LocalAddr(), helpers.HTTPEndpointCases{
		{
			Description: "MAC addresses",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "srcMAC", "prefix": "11:"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "11:33:33:44:55:66", "detail": "MAC address", "quoted": false},
				{"label": "11:22:33:44:55:66", "detail": "MAC address", "quoted": false},
				{"label": "11:FF:33:44:55:66", "detail": "MAC address", "quoted": false},
			}},
		}, {
			Description: "network attributes",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "srcNetName", "prefix": "customer"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "customer-b", "detail": "network name", "quoted": true},
				{"label": "customer-a", "detail": "network name", "quoted": true},
				{"label": "my-customer", "detail": "network name", "quoted": true},
			}},
		}, {
			Description: "exporter names",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "exporterName", "prefix": "th2-"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "th2-router1", "detail": "exporter name", "quoted": true},
				{"label": "th2-router2", "detail": "exporter name", "quoted": true},
				{"label": "edge-th2-1", "detail": "exporter name", "quoted": true},
			}},
		}, {
			Description: "interface names",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "inIfName", "prefix": "gi"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "Gi0/0", "detail": "interface name", "quoted": true},
				{"label": "Gi0/1", "detail": "interface name", "quoted": true},
				{"label": "gi0/2", "detail": "interface name", "quoted": true},
			}},
		}, {
			Description: "custom dictionary attributes",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "dstAddrRole", "prefix": "role"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "role-x", "quoted": true},
				{"label": "a-role", "quoted": true},
				{"label": "b-role", "quoted": true},
			}},
		}, {
			Description: "AS numbers",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "dstAS", "prefix": "goog"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "AS396982", "detail": "Google Cloud", "quoted": false},
				{"label": "AS19527", "detail": "Google", "quoted": false},
				{"label": "AS15169", "detail": "Google", "quoted": false},
				{"label": "AS36040", "detail": "Google", "quoted": false},
				{"label": "AS16550", "detail": "Google Private Cloud", "quoted": false},
				{"label": "AS200000", "detail": "Not Google", "quoted": false},
			}},
		}, {
			Description: "ports",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "dstPort", "prefix": "http"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "8080", "detail": "http-alt", "quoted": false},
				{"label": "443", "detail": "https", "quoted": false},
				{"label": "80", "detail": "http", "quoted": false},
			}},
		}, {
			Description: "ICMPv6",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "icmpv6", "prefix": "echo"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "extended-echo-request", "detail": "ICMPv6", "quoted": true},
				{"label": "echo-reply", "detail": "ICMPv6", "quoted": true},
				{"label": "echo-request", "detail": "ICMPv6", "quoted": true},
				{"label": "extended-echo-reply", "detail": "ICMPv6", "quoted": true},
			}},
		}, {
			Description: "communities",
			URL:         "/api/v0/console/filter/complete",
			StatusCode:  200,
			JSONInput:   helpers.M{"what": "value", "column": "dstCommunities", "prefix": "65401"},
			JSONOutput: helpers.M{"completions": []helpers.M{
				{"label": "65401:12", "detail": "community", "quoted": false},
				{"label": "65401:100:200", "detail": "large community", "quoted": false},
				{"label": "65401:10", "detail": "community", "quoted": false},
			}},
		},
	})
}
