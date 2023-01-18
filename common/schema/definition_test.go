// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"testing"
)

func TestFlowsClickHouse(t *testing.T) {
	c := NewMock(t)
	for _, key := range c.clickHousePrimaryKeys {
		if column := c.columnIndex[key]; column.Key == 0 {
			t.Errorf("primary key %q not a column", key)
		} else {
			if column.ClickHouseNotSortingKey {
				t.Errorf("primary key %q is marked as a non-sorting key", key)
			}
		}
	}
}

func TestFlowsProtobuf(t *testing.T) {
	c := NewMock(t)
	for _, column := range c.Columns() {
		if column.ProtobufIndex >= 0 {
			if column.ProtobufType == 0 {
				t.Errorf("column %s has not protobuf type", column.Name)
			}
		}
	}
}

func TestColumnIndex(t *testing.T) {
	c := NewMock(t)
	for i := ColumnTimeReceived; i < ColumnLast; i++ {
		if _, ok := c.LookupColumnByKey(i); !ok {
			t.Errorf("column %s cannot be looked up by key", i)
		}
	}
}
