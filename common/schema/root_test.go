// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema_test

import (
	"testing"

	"akvorado/common/schema"
)

func TestEnableDisableColumns(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.Enabled = []schema.ColumnKey{schema.ColumnDstVlan, schema.ColumnSrcVlan}
	config.Disabled = []schema.ColumnKey{schema.ColumnSrcCountry, schema.ColumnDstCountry}
	c, err := schema.New(config)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	if column, ok := c.LookupColumnByKey(schema.ColumnDstVlan); !ok {
		t.Fatal("DstVlan not found")
	} else if column.Disabled {
		t.Fatal("DstVlan is still disabled")
	}

	if column, ok := c.LookupColumnByKey(schema.ColumnDstCountry); !ok {
		t.Fatal("DstCountry not found")
	} else if !column.Disabled {
		t.Fatal("DstCountry is not disabled")
	}

}

func TestDisableForbiddenColumns(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.Disabled = []schema.ColumnKey{schema.ColumnDst1stAS}
	if _, err := schema.New(config); err == nil {
		t.Fatal("New() did not error")
	}

	config = schema.DefaultConfiguration()
	config.Disabled = []schema.ColumnKey{schema.ColumnDstLargeCommunitiesASN}
	if _, err := schema.New(config); err == nil {
		t.Fatal("New() did not error")
	}

	config = schema.DefaultConfiguration()
	config.Disabled = []schema.ColumnKey{schema.ColumnDstLargeCommunities}
	if _, err := schema.New(config); err == nil {
		t.Fatal("New() did not error")
	}

	config = schema.DefaultConfiguration()
	config.Disabled = []schema.ColumnKey{
		schema.ColumnDstLargeCommunities,
		schema.ColumnDstLargeCommunitiesASN,
		schema.ColumnDstLargeCommunitiesLocalData1,
		schema.ColumnDstLargeCommunitiesLocalData2,
	}
	if _, err := schema.New(config); err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}
}
