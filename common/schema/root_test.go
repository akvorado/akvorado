// SPDX-FileCopyrightText: 2023 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package schema_test

import (
	"testing"

	"akvorado/common/helpers"
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
}

func TestCustomDictionaries(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{
			{Name: "SrcAddr", Type: "string"},
		},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "string", Label: "DimensionAttribute"},
			{Name: "role", Type: "string"},
		},
		Source:     "test.csv",
		Dimensions: []string{"SrcAddr", "DstAddr"},
	}

	s, err := schema.New(config)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Test if SrcAddrAttribute and DstAddrAttribute are in s.columns
	srcFound := false
	dstFound := false
	srcRoleFound := false
	dstRoleFound := false

	// Check if srcAddrAttribute and dstAddrAttribute are in s.columns, and have the correct type/generatefrom
	for _, column := range s.Columns() {
		if column.Name == "SrcAddrDimensionAttribute" {
			srcFound = true
			if column.ClickHouseType != "LowCardinality(string)" {
				t.Fatalf("SrcAddrDimensionAttribute should be LowCardinality(string), is %s", column.ClickHouseType)
			}
			if column.ClickHouseGenerateFrom != "dictGet('custom_dict_test', 'csv_col_name', SrcAddr)" {
				t.Fatalf("SrcAddrDimensionAttribute should be generated from `dictGet('custom_dict_test', 'csv_col_name', SrcAddr)`, is %s", column.ClickHouseGenerateFrom)
			}
		}
		if column.Name == "DstAddrDimensionAttribute" {
			dstFound = true
			if column.ClickHouseType != "LowCardinality(string)" {
				t.Fatalf("DstAddrDimensionAttribute should be LowCardinality(string), is %s", column.ClickHouseType)
			}
			if column.ClickHouseGenerateFrom != "dictGet('custom_dict_test', 'csv_col_name', DstAddr)" {
				t.Fatalf("DstAddrDimensionAttribute should be generated from `dictGet('custom_dict_test', 'csv_col_name', DstAddr)`, is %s", column.ClickHouseGenerateFrom)
			}
		}
		// This part only tests default dimension name generation
		if column.Name == "SrcAddrRole" {
			srcRoleFound = true
		}
		if column.Name == "DstAddrRole" {
			dstRoleFound = true
		}

	}

	if !srcFound {
		t.Fatal("SrcAddrDimensionAttribute not found")
	}
	if !dstFound {
		t.Fatal("DstAddrDimensionAttribute not found")
	}
	if !srcRoleFound {
		t.Fatal("SrcAddrRole not found")
	}
	if !dstRoleFound {
		t.Fatal("DstAddrRole not found")
	}
}

func TestCustomDictionariesMatcher(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{
			{Name: "exporter", Type: "string", MatchDimension: "ExporterAddress"},
			{Name: "interface", Type: "string", MatchDimensionSuffix: "Name"},
		},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "string", Label: "DimensionAttribute"},
		},
		Source:     "test.csv",
		Dimensions: []string{"OutIf", "InIf"},
		Layout:     "complex_key_hashed",
	}

	s, err := schema.New(config)
	if err != nil {
		t.Fatalf("New() error:\n%+v", err)
	}

	// Test if SrcAddrAttribute and DstAddrAttribute are in s.columns
	outFound := false
	inFound := false

	// Check if srcAddrAttribute and dstAddrAttribute are in s.columns, and have the correct type/generatefrom
	for _, column := range s.Columns() {
		if column.Name == "OutIfDimensionAttribute" {
			outFound = true
			if column.ClickHouseType != "LowCardinality(string)" {
				t.Fatalf("OutIfDimensionAttribute should be LowCardinality(string), is %s", column.ClickHouseType)
			}
			if column.ClickHouseGenerateFrom != "dictGet('custom_dict_test', 'csv_col_name', (ExporterAddress,OutIfName))" {
				t.Fatalf("OutIfDimensionAttribute should be generated from `dictGet('custom_dict_test', 'csv_col_name', (ExporterAddress,OutIfName))`, is %s", column.ClickHouseGenerateFrom)
			}
		}
		if column.Name == "InIfDimensionAttribute" {
			inFound = true
			if column.ClickHouseType != "LowCardinality(string)" {
				t.Fatalf("InIfDimensionAttribute should be LowCardinality(string), is %s", column.ClickHouseType)
			}
			if column.ClickHouseGenerateFrom != "dictGet('custom_dict_test', 'csv_col_name', (ExporterAddress,InIfName))" {
				t.Fatalf("InIfDimensionAttribute should be generated from `dictGet('custom_dict_test', 'csv_col_name', (ExporterAddress,InIfName)), is %s", column.ClickHouseGenerateFrom)
			}
		}
	}

	if !outFound {
		t.Fatal("OutIfDimensionAttribute not found")
	}
	if !inFound {
		t.Fatal("InIfDimensionAttribute not found")
	}
}

// We need MatchDimension or MatchDimensionSuffix for multiple keys
func TestCustomDictMultiKeyErr(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{
			{Name: "exporter", Type: "string"},
			{Name: "interface", Type: "string"},
		},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "string", Label: "DimensionAttribute"},
		},
		Source:     "test.csv",
		Dimensions: []string{"OutIf", "InIf"},
		Layout:     "complex_key_hashed",
	}

	_, err := schema.New(config)
	if err == nil {
		t.Fatal("New() did not error")
	}

	if diff := helpers.Diff(err.Error(), "custom dictionary test has more than one key, but key exporter has neither MatchDimension nor MatchDimensionSuffix set"); diff != "" {
		t.Fatalf("New() did not error correctly\n %s", diff)
	}
}

// A dict without key makes no sense, catch this
func TestCustomDictNoKeyErr(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.CustomDictionaries = make(map[string]schema.CustomDict)
	config.CustomDictionaries["test"] = schema.CustomDict{
		Keys: []schema.CustomDictKey{},
		Attributes: []schema.CustomDictAttribute{
			{Name: "csv_col_name", Type: "string", Label: "DimensionAttribute"},
		},
		Source:     "test.csv",
		Dimensions: []string{"OutIf", "InIf"},
		Layout:     "complex_key_hashed",
	}

	_, err := schema.New(config)
	if err == nil {
		t.Fatal("New() did not error")
	}

	if diff := helpers.Diff(err.Error(), "custom dictionary test has no keys, this is not supported"); diff != "" {
		t.Fatalf("New() did not error correctly\n %s", diff)
	}
}
