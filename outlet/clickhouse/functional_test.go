// SPDX-FileCopyrightText: 2025 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"

	"akvorado/common/clickhousedb"
	"akvorado/common/daemon"
	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
	"akvorado/outlet/clickhouse"
)

func TestInsert(t *testing.T) {
	server := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse:9000", "127.0.0.1:9000"})
	r := reporter.NewMock(t)
	sch := schema.NewMock(t)
	bf := sch.NewFlowMessage()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = clickhousego.Context(ctx, clickhousego.WithSettings(clickhousego.Settings{
		"allow_suspicious_low_cardinality_types": 1,
	}))

	// Create components
	dbConf := clickhousedb.DefaultConfiguration()
	dbConf.Servers = []string{server}
	dbConf.Database = "test"
	dbConf.DialTimeout = 100 * time.Millisecond
	chdb, err := clickhousedb.New(r, dbConf, clickhousedb.Dependencies{
		Daemon: daemon.NewMock(t),
	})
	if err != nil {
		t.Fatalf("clickhousedb.New() error:\n%+v", err)
	}
	helpers.StartStop(t, chdb)
	conf := clickhouse.DefaultConfiguration()
	conf.MaximumBatchSize = 10
	conf.MaximumWaitTime = time.Second
	ch, err := clickhouse.New(r, conf, clickhouse.Dependencies{
		ClickHouse: chdb,
		Schema:     sch,
	})
	if err != nil {
		t.Fatalf("clickhouse.New() error:\n%+v", err)
	}
	helpers.StartStop(t, ch)

	// Create table
	tableName := fmt.Sprintf("flows_%s_raw", sch.ClickHouseHash())
	err = chdb.Exec(ctx, fmt.Sprintf("CREATE OR REPLACE TABLE %s (%s) ENGINE = Memory", tableName,
		sch.ClickHouseCreateTable(
			schema.ClickHouseSkipGeneratedColumns,
			schema.ClickHouseSkipAliasedColumns)))
	if err != nil {
		t.Fatalf("chdb.Exec() error:\n%+v", err)
	}
	// Drop any left-over consumer (from orchestrator tests). Otherwise, we get an error like this:
	// Bad URI syntax: bad or invalid port number: 0
	err = chdb.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s_consumer", tableName))
	if err != nil {
		t.Fatalf("chdb.Exec() error:\n%+v", err)
	}

	// Expected records
	type result struct {
		TimeReceived time.Time
		SrcAS        uint32
		DstAS        uint32
		ExporterName string
		EType        uint32
	}
	expected := []result{}

	// Create one worker and send some values
	w := ch.NewWorker(1, bf)
	for i := range 23 {
		i = i + 1
		// 1: first batch (max time)
		// 2 to 11: second batch (max batch)
		// 12 to 15: third batch (max time)
		// 16 to 23: third batch (last one)
		bf.TimeReceived = uint32(100 + i)
		bf.SrcAS = uint32(65400 + i)
		bf.DstAS = uint32(65500 + i)
		bf.AppendString(schema.ColumnExporterName, fmt.Sprintf("exporter-%d", i))
		bf.AppendString(schema.ColumnExporterName, "emptyness")
		bf.AppendUint(schema.ColumnEType, helpers.ETypeIPv6)
		expected = append(expected, result{
			TimeReceived: time.Unix(int64(bf.TimeReceived), 0).UTC(),
			SrcAS:        bf.SrcAS,
			DstAS:        bf.DstAS,
			ExporterName: fmt.Sprintf("exporter-%d", i),
			EType:        helpers.ETypeIPv6,
		})
		if i == 15 {
			time.Sleep(time.Second)
		}
		w.FinalizeAndSend(ctx)
		if i == 23 {
			w.Flush(ctx)
		}

		// Check metrics
		gotMetrics := r.GetMetrics("akvorado_outlet_clickhouse_", "-insert_time", "-wait_time")
		var expectedMetrics map[string]string
		if i < 11 {
			expectedMetrics = map[string]string{
				`flow_per_batch_count`:            "1",
				`flow_per_batch_sum`:              "1",
				`flow_per_batch{quantile="0.5"}`:  "1",
				`flow_per_batch{quantile="0.9"}`:  "1",
				`flow_per_batch{quantile="0.99"}`: "1",
				`worker_overloaded_total`:         "0",
				`worker_underloaded_total`:        "1", // only the first one is "underloaded"
			}
		} else if i < 15 {
			expectedMetrics = map[string]string{
				`flow_per_batch_count`:            "2",
				`flow_per_batch_sum`:              "11",
				`flow_per_batch{quantile="0.5"}`:  "1",
				`flow_per_batch{quantile="0.9"}`:  "10",
				`flow_per_batch{quantile="0.99"}`: "10",
				`worker_overloaded_total`:         "1", // full batch size
				`worker_underloaded_total`:        "1",
			}
		} else if i < 23 {
			expectedMetrics = map[string]string{
				`flow_per_batch_count`:            "3",
				`flow_per_batch_sum`:              "15",
				`flow_per_batch{quantile="0.5"}`:  "4",
				`flow_per_batch{quantile="0.9"}`:  "10",
				`flow_per_batch{quantile="0.99"}`: "10",
				`worker_overloaded_total`:         "1",
				`worker_underloaded_total`:        "1",
			}
		} else {
			expectedMetrics = map[string]string{
				`flow_per_batch_count`:            "4",
				`flow_per_batch_sum`:              "23",
				`flow_per_batch{quantile="0.5"}`:  "4",
				`flow_per_batch{quantile="0.9"}`:  "10",
				`flow_per_batch{quantile="0.99"}`: "10",
				`worker_overloaded_total`:         "1",
				`worker_underloaded_total`:        "1",
			}
		}
		if diff := helpers.Diff(gotMetrics, expectedMetrics); diff != "" {
			t.Errorf("Metrics, iteration %d, (-got, +want):\n%s", i, diff)
		}

		// Check if we have anything inserted in the table
		var results []result
		err := chdb.Select(ctx, &results,
			fmt.Sprintf("SELECT TimeReceived, SrcAS, DstAS, ExporterName, EType FROM %s ORDER BY TimeReceived ASC", tableName))
		if err != nil {
			t.Fatalf("chdb.Select() error:\n%+v", err)
		}
		reallyExpected := expected
		if i < 11 {
			reallyExpected = expected[:min(len(expected), 1)]
		} else if i < 15 {
			reallyExpected = expected[:min(len(expected), 11)]
		} else if i < 23 {
			reallyExpected = expected[:min(len(expected), 15)]
		}
		if diff := helpers.Diff(results, reallyExpected); diff != "" {
			t.Fatalf("chdb.Select(), iteration %d, (-got, +want):\n%s", i, diff)
		}
	}
}

func TestMultipleServers(t *testing.T) {
	servers := []string{
		helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse:9000", "127.0.0.1:9000"}),
	}
	for range 100 {
		servers = append(servers, "127.0.0.1:0")
	}
	for range 10 {
		r := reporter.NewMock(t)
		sch := schema.NewMock(t)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		ctx = clickhousego.Context(ctx, clickhousego.WithSettings(clickhousego.Settings{
			"allow_suspicious_low_cardinality_types": 1,
		}))

		// Create components
		dbConf := clickhousedb.DefaultConfiguration()
		dbConf.Servers = servers
		dbConf.DialTimeout = 100 * time.Millisecond
		chdb, err := clickhousedb.New(r, dbConf, clickhousedb.Dependencies{
			Daemon: daemon.NewMock(t),
		})
		if err != nil {
			t.Fatalf("clickhousedb.New() error:\n%+v", err)
		}
		helpers.StartStop(t, chdb)
		conf := clickhouse.DefaultConfiguration()
		conf.MaximumBatchSize = 10
		ch, err := clickhouse.New(r, conf, clickhouse.Dependencies{
			ClickHouse: chdb,
			Schema:     sch,
		})
		if err != nil {
			t.Fatalf("clickhouse.New() error:\n%+v", err)
		}
		helpers.StartStop(t, ch)

		// Trigger an empty send
		bf := sch.NewFlowMessage()
		w := ch.NewWorker(1, bf)
		w.Flush(ctx)

		// Check metrics
		gotMetrics := r.GetMetrics("akvorado_outlet_clickhouse_", "errors_total")
		if gotMetrics[`errors_total{error="connect"}`] == "0" {
			continue
		}
		return
	}
	t.Fatalf("w.Flush(): cannot trigger connect error")
}
