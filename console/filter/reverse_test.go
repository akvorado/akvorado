// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"fmt"
	"testing"

	"akvorado/common/helpers"
	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
)

func TestReverse(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
	}{
		{Input: ``, Expected: ``},
		{Input: `SrcAS = AS12322`, Expected: `DstAS = AS12322`},
		{Input: `srcas = AS12322`, Expected: `DstAS = AS12322`},
		{Input: `InIfBoundary = external`, Expected: `OutIfBoundary = external`},
		{
			Input:    `InIfProvider = OutIfProvider AND NOT (SrcAddr << 192.0.2.0/24 OR DstPort IN (80, 443))`,
			Expected: `OutIfProvider = InIfProvider AND NOT (DstAddr << 192.0.2.0/24 OR SrcPort IN (80, 443))`,
		},
		{Input: `SrcPort>1024 -- SrcPort`, Expected: `DstPort>1024 -- SrcPort`},
		{Input: `DstASPath = AS65000`, Expected: `DstASPath = AS65000`},
		{Input: `ForwardingStatus >= 128`, Expected: `ForwardingStatus >= 128`},
		{Input: `exportername = 'th2'`, Expected: `exportername = 'th2'`},
		{Input: `/* SrcAS */ DstCommunities = 65000:100:200`, Expected: `/* SrcAS */ SrcCommunities = 65000:100:200`},
		{Input: `ExporterName = "SrcAS"`, Expected: `ExporterName = "SrcAS"`},
	}
	s := schema.NewMock(t).EnableAllColumns()
	for _, tc := range cases {
		got, err := Reverse(tc.Input, s)
		if err != nil {
			t.Errorf("Reverse(%q) error:\n%+v", tc.Input, err)
			continue
		}
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("Reverse(%q) (-got, +want):\n%s", tc.Input, diff)
		}
	}
}

func TestReverseInvalid(t *testing.T) {
	s := schema.NewMock(t).EnableAllColumns()
	if _, err := Reverse(`SrcAS = `, s); err == nil {
		t.Error("Reverse() did not error")
	}
}

func TestReverseDisabledOpposite(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.Enabled = []schema.ColumnKey{schema.ColumnSrcMAC}
	s, err := schema.New(config)
	if err != nil {
		t.Fatalf("schema.New() error:\n%+v", err)
	}
	input := `SrcMAC = 00:11:22:33:44:55`
	got, err := Reverse(input, s)
	if err != nil {
		t.Fatalf("Reverse(%q) error:\n%+v", input, err)
	}
	if diff := helpers.Diff(got, input); diff != "" {
		t.Errorf("Reverse(%q) (-got, +want):\n%s", input, diff)
	}
}

func TestParseReverseDirectionDisabledOpposite(t *testing.T) {
	config := schema.DefaultConfiguration()
	config.Enabled = []schema.ColumnKey{schema.ColumnSrcMAC}
	s, err := schema.New(config)
	if err != nil {
		t.Fatalf("schema.New() error:\n%+v", err)
	}
	input := `SrcMAC = 00:11:22:33:44:55`
	expr, err := Parse("", []byte(input),
		GlobalStore("meta", &Meta{Schema: s, ReverseDirection: true}))
	if err != nil {
		t.Fatalf("Parse(%q) error:\n%+v", input, err)
	}
	if diff := helpers.Diff(expr.(sb.Expr).String(), `SrcMAC = MACStringToNum('00:11:22:33:44:55')`); diff != "" {
		t.Errorf("Parse(%q) (-got, +want):\n%s", input, diff)
	}
}

// TestReverseSameAsReverseDirection checks a reversed filter gives the same SQL
// as the original filter parsed for the reverse direction.
func TestReverseSameAsReverseDirection(t *testing.T) {
	values := map[string]string{
		"array(uint)": "100",
		"asn":         "AS65000",
		"aspath":      "AS65000",
		"boundary":    "external",
		"community":   "65000:100",
		"direction":   "ingress",
		"etype":       "IPv4",
		"ip":          "192.0.2.0/24",
		"mac":         "00:11:22:33:44:55",
		"prefix":      "192.0.2.0/24",
		"proto":       `"TCP"`,
		"string":      `"something"`,
		"uint":        "100",
	}
	inputs := []string{
		`InIfProvider = OutIfProvider AND NOT (SrcAddr << 192.0.2.0/24 OR DstPort IN (80, 443))`,
		`SrcAS = DstAS OR InIfSpeed > OutIfSpeed`,
		`DstCommunities = 65000:100:200 AND SrcNetName != DstNetName`,
		`srcport = 80 /* SrcPort */ -- DstPort`,
	}
	s := schema.NewMock(t).EnableAllColumns()
	for _, column := range s.Columns() {
		if column.ParserType == "" {
			continue
		}
		value, ok := values[column.ParserType]
		if !ok {
			t.Errorf("no value for column %s of type %q", column.Name, column.ParserType)
			continue
		}
		inputs = append(inputs, fmt.Sprintf("%s = %s", column.Name, value))
	}

	parse := func(input string, reverse bool) string {
		t.Helper()
		got, err := Parse("", []byte(input),
			GlobalStore("meta", &Meta{Schema: s, ReverseDirection: reverse}))
		if err != nil {
			t.Fatalf("Parse(%q) error:\n%+v", input, err)
		}
		return got.(sb.Expr).String()
	}
	for _, input := range inputs {
		reversed, err := Reverse(input, s)
		if err != nil {
			t.Errorf("Reverse(%q) error:\n%+v", input, err)
			continue
		}
		if diff := helpers.Diff(parse(reversed, false), parse(input, true)); diff != "" {
			t.Errorf("Reverse(%q) SQL (-got, +want):\n%s", input, diff)
		}
	}
}
