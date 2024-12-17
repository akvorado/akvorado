// SPDX-FileCopyrightText: 2016-2023 ClickHouse, Inc.
// SPDX-License-Identifier: Apache-2.0
// SPDX-FileComment: This is basically a copy of https://github.com/ClickHouse/ch-go/blob/main/examples/insert/main.go

package clickhouse_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/ClickHouse/ch-go"
	"github.com/ClickHouse/ch-go/proto"

	"akvorado/common/helpers"
)

func TestInsertMemory(t *testing.T) {
	server := helpers.CheckExternalService(t, "ClickHouse", []string{"clickhouse:9000", "127.0.0.1:9000"})
	ctx := context.Background()

	conn, err := ch.Dial(ctx, ch.Options{
		Address:     server,
		DialTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Dial() error:\n%+v", err)
	}

	if err := conn.Do(ctx, ch.Query{
		Body: `CREATE OR REPLACE TABLE test_table_insert
(
    ts                DateTime64(9),
    severity_text     Enum8('INFO'=1, 'DEBUG'=2),
    severity_number   UInt8,
	service_name      LowCardinality(String),
    body              String,
    name              String,
    arr               Array(String)
) ENGINE = Memory`,
	}); err != nil {
		t.Fatalf("Do() error:\n%+v", err)
	}

	// Define all columns of table.
	var (
		body      proto.ColStr
		name      proto.ColStr
		sevText   proto.ColEnum
		sevNumber proto.ColUInt8

		// or new(proto.ColStr).LowCardinality()
		serviceName = proto.NewLowCardinality(new(proto.ColStr))
		ts          = new(proto.ColDateTime64).WithPrecision(proto.PrecisionNano) // DateTime64(9)
		arr         = new(proto.ColStr).Array()                                   // Array(String)
		now         = time.Date(2010, 1, 1, 10, 22, 33, 345678, time.UTC)
	)

	input := proto.Input{
		{Name: "ts", Data: ts},
		{Name: "severity_text", Data: &sevText},
		{Name: "severity_number", Data: &sevNumber},
		{Name: "service_name", Data: serviceName},
		{Name: "body", Data: &body},
		{Name: "name", Data: &name},
		{Name: "arr", Data: arr},
	}

	t.Run("one block", func(t *testing.T) {
		// Append 10 rows to initial data block.
		for range 10 {
			body.AppendBytes([]byte("Hello"))
			ts.Append(now)
			name.Append("name")
			sevText.Append("INFO")
			sevNumber.Append(10)
			arr.Append([]string{"foo", "bar", "baz"})
			serviceName.Append("service")
		}

		// Insert single data block.
		if err := conn.Do(ctx, ch.Query{
			Body:  "INSERT INTO test_table_insert VALUES",
			Input: input,
		}); err != nil {
			t.Fatalf("Do() error:\n%+v", err)
		}
	})

	t.Run("streaming", func(t *testing.T) {
		// Stream data to ClickHouse server in multiple data blocks.
		var blocks int
		if err := conn.Do(ctx, ch.Query{
			Body:  input.Into("test_table_insert"), // helper that generates INSERT INTO query with all columns
			Input: input,

			// OnInput is called to prepare Input data before encoding and sending
			// to ClickHouse server.
			OnInput: func(ctx context.Context) error {
				// On OnInput call, you should fill the input data.
				//
				// NB: You should reset the input columns, they are
				// not reset automatically.
				//
				// That is, we are re-using the same input columns and
				// if we will return nil without doing anything, data will be
				// just duplicated.

				input.Reset() // calls "Reset" on each column

				if blocks >= 10 {
					// Stop streaming.
					//
					// This will also write tailing input data if any,
					// but we just reset the input, so it is currently blank.
					return io.EOF
				}

				// Append new values:
				for range 10 {
					body.AppendBytes([]byte("Hello"))
					ts.Append(now)
					name.Append("name")
					sevText.Append("DEBUG")
					sevNumber.Append(10)
					arr.Append([]string{"foo", "bar", "baz"})
					serviceName.Append("service")
				}

				// Data will be encoded and sent to ClickHouse server after returning nil.
				// The Do method will return error if any.
				blocks++
				return nil
			},
		}); err != nil {
			t.Fatalf("Do() error:\n%+v", err)
		}
	})
}
