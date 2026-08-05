// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package console

import (
	"fmt"
	"testing"
	"time"

	"akvorado/common/schema"
	sb "akvorado/common/sqlbuilder"
	"akvorado/console/query"
)

var (
	testStart = time.Date(2022, 4, 10, 15, 45, 10, 0, time.UTC)
	testEnd   = time.Date(2022, 4, 11, 15, 45, 10, 0, time.UTC)
)

func TestColumnSQLSelectParses(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()
	tested := 0
	for _, column := range sch.Columns() {
		qc := query.NewColumn(column.Name)
		if err := qc.Validate(sch); err != nil {
			// Not usable as a dimension
			continue
		}
		tested++
		t.Run(column.Name, func(t *testing.T) {
			sb.CheckStatement(t, fmt.Sprintf("SELECT %s", qc.ToSQLSelect(sch)))
		})
	}
	if tested < 20 {
		t.Errorf("only %d columns tested, expected the whole schema", tested)
	}
}

func TestGeneratedQueriesParse(t *testing.T) {
	sch := schema.NewMock(t).EnableAllColumns()

	// Each variant exercises a different part of the query shape. They are
	// combined with the flags below rather than crossed with each other, to
	// keep the number of queries reasonable.
	variants := []struct {
		Description string
		Dimensions  []query.Column
		Filter      string
		LimitType   string
		Units       string
		Truncate    bool
	}{
		{Description: "no dimension", Units: "l3bps"},
		{
			Description: "one dimension",
			Dimensions:  []query.Column{query.NewColumn("ExporterName")},
			Units:       "fps",
		}, {
			Description: "two dimensions, truncated addresses",
			Dimensions:  []query.Column{query.NewColumn("SrcAddr"), query.NewColumn("DstAS")},
			Units:       "pps",
			Truncate:    true,
		}, {
			// Lambdas, hexadecimal literals and newlines inside one expression
			Description: "communities",
			Dimensions:  []query.Column{query.NewColumn("SrcCommunities")},
			Units:       "l2bps",
		}, {
			// Dictionary lookups and a regular expression
			Description: "ports and protocol",
			Dimensions:  []query.Column{query.NewColumn("SrcPort"), query.NewColumn("Proto")},
			Units:       "inl2%",
		}, {
			Description: "TCP flags",
			Dimensions:  []query.Column{query.NewColumn("TCPFlags")},
			Units:       "outl2%",
		}, {
			Description: "filter on strings",
			Dimensions:  []query.Column{query.NewColumn("ExporterName")},
			Filter:      "SrcCountry = 'FR' AND DstCountry != 'US'",
			LimitType:   "max",
			Units:       "l3bps",
		}, {
			// Turns into an IN plus a BETWEEN disjunction
			Description: "filter on addresses and subnets",
			Dimensions:  []query.Column{query.NewColumn("SrcAddr")},
			Filter:      "SrcAddr IN (192.0.2.0/24, 203.0.113.1)",
			LimitType:   "last",
			Units:       "l3bps",
		}, {
			Description: "filter needing the main table",
			Dimensions:  []query.Column{query.NewColumn("ExporterName")},
			Filter:      "SrcPort = 80",
			Units:       "l3bps",
		}, {
			Description: "filter with quotes and backslashes",
			Dimensions:  []query.Column{query.NewColumn("ExporterName")},
			Filter:      `InIfDescription = 'a backslash \\ and a quote \''`,
			Units:       "l3bps",
		}, {
			Description: "filter with braces",
			Dimensions:  []query.Column{query.NewColumn("ExporterName")},
			Filter:      "InIfDescription != '{{ hello }}'",
			Units:       "l3bps",
		},
	}

	for _, variant := range variants {
		for _, bidirectional := range []bool{false, true} {
			for _, previousPeriod := range []bool{false, true} {
				name := fmt.Sprintf("%s-b%v-p%v", variant.Description, bidirectional, previousPeriod)
				t.Run(name, func(t *testing.T) {
					common := graphCommonHandlerInput{
						schema:     sch,
						Start:      testStart,
						End:        testEnd,
						Dimensions: variant.Dimensions,
						Limit:      10,
						LimitType:  variant.LimitType,
						Filter:     query.NewFilter(variant.Filter),
						Units:      variant.Units,
					}
					if variant.Truncate {
						common.TruncateAddrV4 = 24
						common.TruncateAddrV6 = 48
					}
					if err := query.Columns(common.Dimensions).Validate(sch); err != nil {
						t.Fatalf("Validate() error:\n%+v", err)
					}
					if err := common.Filter.Validate(sch); err != nil {
						t.Fatalf("Validate() error:\n%+v", err)
					}

					line := graphLineHandlerInput{
						graphCommonHandlerInput: common,
						Points:                  100,
						Bidirectional:           bidirectional,
						PreviousPeriod:          previousPeriod,
					}
					sb.CheckStatement(t, unionAll(line.toSQL(testResolution)))

					if len(variant.Dimensions) == 0 {
						// The sankey handler rejects those
						return
					}
					sankey := graphSankeyHandlerInput{
						graphCommonHandlerInput: common,
						Bidirectional:           bidirectional,
					}
					sb.CheckStatement(t, unionAll(sankey.toSQL(testResolution)))
				})
			}
		}
	}
}

// toSQLStrings renders the per-axis queries, so the tests can pin the SQL each
// axis produces. The result is normalized, like the expected SQL it is compared
// with.
func toSQLStrings(t *testing.T, queries []*sb.Query) []string {
	t.Helper()
	sql := make([]string, len(queries))
	for i, query := range queries {
		sql[i] = sb.Normalize(t, query.String())
	}
	return sql
}
