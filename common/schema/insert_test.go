// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/helpers"
	"akvorado/common/schema"
)

func TestInsertMemory(t *testing.T) {
	c := schema.NewMock(t)
	bf := c.NewFlowMessage()
	exporterAddress := netip.MustParseAddr("::ffff:203.0.113.14")

	bf.TimeReceived = 1000
	bf.SamplingRate = 20000
	bf.ExporterAddress = exporterAddress
	bf.AppendString(schema.ColumnExporterName, "router1.example.net")
	bf.AppendUint(schema.ColumnSrcAS, 65000)
	bf.AppendUint(schema.ColumnDstAS, 12322)
	bf.AppendUint(schema.ColumnBytes, 20)
	bf.AppendUint(schema.ColumnPackets, 3)
	bf.AppendUint(schema.ColumnInIfBoundary, uint64(schema.InterfaceBoundaryInternal))
	bf.AppendUint(schema.ColumnOutIfBoundary, uint64(schema.InterfaceBoundaryExternal))
	bf.AppendUint(schema.ColumnInIfSpeed, 10000)
	bf.AppendUint(schema.ColumnEType, helpers.ETypeIPv4)
	bf.Finalize()

	bf.TimeReceived = 1001
	bf.SamplingRate = 20000
	bf.ExporterAddress = exporterAddress
	bf.AppendString(schema.ColumnExporterName, "router1.example.net")
	bf.AppendUint(schema.ColumnSrcAS, 12322)
	bf.AppendUint(schema.ColumnDstAS, 65000)
	bf.AppendUint(schema.ColumnBytes, 200)
	bf.AppendUint(schema.ColumnPackets, 3)
	bf.AppendUint(schema.ColumnInIfBoundary, uint64(schema.InterfaceBoundaryExternal))
	bf.AppendUint(schema.ColumnOutIfSpeed, 10000)
	bf.AppendUint(schema.ColumnEType, helpers.ETypeIPv4)
	bf.AppendArrayUInt32(schema.ColumnDstASPath, []uint32{65400, 65500, 65001})
	bf.AppendArrayUInt128(schema.ColumnDstLargeCommunities, []schema.UInt128{
		{
			High: 65401,
			Low:  (100 << 32) + 200,
		},
		{
			High: 65401,
			Low:  (100 << 32) + 201,
		},
	})
	bf.Finalize()

	server := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse:9000", "127.0.0.1:9000"})
	ctx := t.Context()

	conn, err := ch.Dial(ctx, ch.Options{
		Address:     server,
		Database:    "test",
		DialTimeout: 100 * time.Millisecond,
		Settings: []ch.Setting{
			{Key: "allow_suspicious_low_cardinality_types", Value: "1"},
		},
	})
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	// Create the table
	q := fmt.Sprintf(
		`CREATE OR REPLACE TABLE test_schema_insert (%s) ENGINE = Memory`,
		c.ClickHouseCreateTable(schema.ClickHouseSkipAliasedColumns, schema.ClickHouseSkipGeneratedColumns),
	)
	t.Logf("Query: %s", q)
	if err := conn.Do(ctx, ch.Query{
		Body: q,
	}); err != nil {
		t.Fatalf("Do() error:\n%+v", err)
	}

	// Insert
	input := bf.ClickHouseProtoInput()
	if err := conn.Do(ctx, ch.Query{
		Body:  input.Into("test_schema_insert"),
		Input: input,
		OnInput: func(ctx context.Context) error {
			bf.Clear()
			// No more data to send!
			return io.EOF
		},
	}); err != nil {
		t.Fatalf("Do() error:\n%+v", err)
	}

	// Check the result (with the full-featured client)
	{
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{server},
			Auth: clickhouse.Auth{
				Database: "test",
			},
			DialTimeout: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("clickhouse.Open() error:\n%+v", err)
		}
		// Use formatRow to get JSON representation
		rows, err := conn.Query(ctx, "SELECT formatRow('JSONEachRow', *) FROM test_schema_insert ORDER BY TimeReceived")
		if err != nil {
			t.Fatalf("clickhouse.Query() error:\n%+v", err)
		}

		var got []map[string]any
		for rows.Next() {
			var jsonRow string
			if err := rows.Scan(&jsonRow); err != nil {
				t.Fatalf("rows.Scan() error:\n%+v", err)
			}

			var row map[string]any
			if err := json.Unmarshal([]byte(jsonRow), &row); err != nil {
				t.Fatalf("json.Unmarshal() error:\n%+v", err)
			}

			// Remove fields with default values
			for k, v := range row {
				switch val := v.(type) {
				case string:
					if val == "" || val == "::" {
						delete(row, k)
					}
				case float64:
					if val == 0 {
						delete(row, k)
					}
				case []any:
					if len(val) == 0 {
						delete(row, k)
					}
				}
			}
			got = append(got, row)
		}
		rows.Close()

		expected := []map[string]any{
			{
				"TimeReceived":    "1970-01-01 00:16:40",
				"SamplingRate":    "20000",
				"ExporterAddress": "::ffff:203.0.113.14",
				"ExporterName":    "router1.example.net",
				"SrcAS":           float64(65000),
				"DstAS":           float64(12322),
				"Bytes":           "20",
				"Packets":         "3",
				"InIfBoundary":    "internal",
				"OutIfBoundary":   "external",
				"InIfSpeed":       float64(10000),
				"EType":           float64(helpers.ETypeIPv4),
			}, {
				"TimeReceived":    "1970-01-01 00:16:41",
				"SamplingRate":    "20000",
				"ExporterAddress": "::ffff:203.0.113.14",
				"ExporterName":    "router1.example.net",
				"SrcAS":           float64(12322),
				"DstAS":           float64(65000),
				"Bytes":           "200",
				"Packets":         "3",
				"InIfBoundary":    "external",
				"OutIfBoundary":   "undefined",
				"OutIfSpeed":      float64(10000),
				"EType":           float64(helpers.ETypeIPv4),
				"DstASPath":       []any{float64(65400), float64(65500), float64(65001)},
				"DstLargeCommunities": []any{
					"1206435509165107881967816", // 65401:100:200
					"1206435509165107881967817", // 65401:100:201
				},
			},
		}

		if diff := helpers.Diff(got, expected); diff != "" {
			t.Errorf("Insert (-got, +want):\n%s", diff)
		}
	}
}
