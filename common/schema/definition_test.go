// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"testing"

	"akvorado/common/helpers"

	"github.com/bits-and-blooms/bitset"
	"github.com/google/go-cmp/cmp"
)

func TestFlowsClickHouse(t *testing.T) {
	c := NewMock(t)
	for _, key := range c.clickhousePrimaryKeys {
		if column := c.columnIndex[key]; column.Key == 0 {
			t.Errorf("primary key %q not a column", key)
		} else {
			if column.ClickHouseNotSortingKey {
				t.Errorf("primary key %q is marked as a non-sorting key", key)
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

func TestFinalizeTwice(t *testing.T) {
	c := NewMock(t)
	oldSchema := c.Schema
	newSchema := c.finalize()
	if diff := helpers.Diff(oldSchema, newSchema,
		cmp.AllowUnexported(Schema{}),
		cmp.Comparer(func(x, y bitset.BitSet) bool {
			return x.Equal(&y)
		})); diff != "" {
		t.Fatalf("finalize() (-old, +new):\n%s", diff)
	}
}

func TestDisabledGroup(t *testing.T) {
	c := flows()
	if !c.IsDisabled(ColumnGroupNAT) {
		t.Error("ColumnGroupNAT is not disabled while it should")
	}
	if !c.IsDisabled(ColumnGroupL2) {
		t.Error("ColumnGroupL2 is not disabled while it should")
	}
	column, _ := c.LookupColumnByKey(ColumnSrcAddrNAT)
	column.Disabled = false
	c = c.finalize()
	if c.IsDisabled(ColumnGroupNAT) {
		t.Error("ColumnGroupNAT is disabled while it should not")
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	interfaceBoundaryMap.TestMarshalUnmarshal(t)
	columnNameMap.TestMarshalUnmarshal(t)
}
