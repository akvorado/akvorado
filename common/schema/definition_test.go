// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"testing"
)

func TestFlowsClickHouse(t *testing.T) {
	for _, key := range Flows.clickHousePrimaryKeys {
		if column := Flows.columnIndex[key]; column.Key == 0 {
			t.Errorf("primary key %q not a column", key)
		} else {
			if column.ClickHouseNotSortingKey {
				t.Errorf("primary key %q is marked as a non-sorting key", key)
			}
		}
	}
}

func TestFlowsProtobuf(t *testing.T) {
	for _, column := range Flows.Columns() {
		if column.ProtobufIndex >= 0 {
			if column.ProtobufType == 0 {
				t.Errorf("column %s has not protobuf type", column.Name)
			}
		}
	}
}

func TestColumnIndex(t *testing.T) {
	for i := ColumnTimeReceived; i < ColumnLast; i++ {
		if _, ok := Flows.LookupColumnByKey(i); !ok {
			t.Errorf("column %s cannot be looked up by key", i)
		}
	}
}
