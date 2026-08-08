// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder_test

import (
	"testing"

	"akvorado/common/helpers"
	sb "akvorado/common/sqlbuilder"
)

func TestExpr(t *testing.T) {
	cases := []struct {
		Description string
		Expr        sb.Expr
		Expected    string
	}{
		{
			Description: "column",
			Expr:        sb.Column("SrcAddr"),
			Expected:    "SrcAddr",
		}, {
			Description: "string with a quote",
			Expr:        sb.String(`it's`),
			Expected:    `'it\'s'`,
		}, {
			Description: "string with a backslash",
			Expr:        sb.String(`a\b`),
			Expected:    `'a\\b'`,
		}, {
			Description: "function",
			Expr:        sb.Function("toIPv6", sb.String("::1")),
			Expected:    `toIPv6('::1')`,
		}, {
			Description: "comparison",
			Expr:        sb.Op(sb.Column("SrcAS"), "=", sb.Uint(65000)),
			Expected:    "SrcAS = 65000",
		}, {
			Description: "negated operator",
			Expr: sb.Op(sb.Column("ExporterName"), "NOT LIKE",
				sb.String("th%")),
			Expected: "ExporterName NOT LIKE 'th%'",
		}, {
			Description: "cast",
			Expr:        sb.Cast(sb.Uint(65000), "UInt128"),
			Expected:    "65000::UInt128",
		}, {
			Description: "and",
			Expr: sb.And(
				sb.Op(sb.Column("a"), "=", sb.Uint(1)),
				sb.Op(sb.Column("b"), "=", sb.Uint(2)),
				sb.Op(sb.Column("c"), "=", sb.Uint(3))),
			Expected: "a = 1 AND b = 2 AND c = 3",
		}, {
			Description: "and skips empty expressions",
			Expr: sb.And(
				sb.Expr{},
				sb.Op(sb.Column("a"), "=", sb.Uint(1)),
				sb.Expr{}),
			Expected: "a = 1",
		}, {
			// OR binds less tightly, so it needs parentheses inside an AND.
			Description: "or inside and",
			Expr: sb.And(
				sb.Op(sb.Column("a"), "=", sb.Uint(1)),
				sb.Or(
					sb.Op(sb.Column("b"), "=", sb.Uint(2)),
					sb.Op(sb.Column("c"), "=", sb.Uint(3)))),
			Expected: "a = 1 AND (b = 2 OR c = 3)",
		}, {
			Description: "and inside or",
			Expr: sb.Or(
				sb.Op(sb.Column("a"), "=", sb.Uint(1)),
				sb.And(
					sb.Op(sb.Column("b"), "=", sb.Uint(2)),
					sb.Op(sb.Column("c"), "=", sb.Uint(3)))),
			Expected: "a = 1 OR b = 2 AND c = 3",
		}, {
			Description: "not",
			Expr: sb.Not(sb.Op(sb.Column("a"), "=",
				sb.Uint(1))),
			Expected: "NOT (a = 1)",
		}, {
			Description: "between",
			Expr: sb.Between(sb.Column("SrcAddr"),
				sb.Function("toIPv6", sb.String("::1")),
				sb.Function("toIPv6", sb.String("::2"))),
			Expected: `SrcAddr BETWEEN toIPv6('::1') AND toIPv6('::2')`,
		}, {
			Description: "in a tuple",
			Expr: sb.Op(sb.Column("Proto"), "IN",
				sb.Tuple(sb.Uint(6), sb.Uint(17))),
			Expected: "Proto IN (6, 17)",
		}, {
			Description: "array",
			Expr:        sb.Array(sb.String("a"), sb.String("b")),
			Expected:    `['a', 'b']`,
		}, {
			Description: "interval arithmetic",
			Expr: sb.Op(sb.Column("TimeReceived"), "+",
				sb.Interval(sb.Uint(60), "second")),
			Expected: "TimeReceived + INTERVAL 60 second",
		}, {
			Description: "alias",
			Expr:        sb.Alias(sb.Column("TimeReceived"), "time"),
			Expected:    "TimeReceived AS time",
		}, {
			Description: "raw expression",
			Expr:        sb.MustParseExpr("SUM(Bytes*SamplingRate*8)"),
			Expected:    "SUM(Bytes * SamplingRate * 8)",
		}, {
			Description: "raw expression with a lambda",
			Expr:        sb.MustParseExpr("arrayMap(c -> bitAnd(c, 0xffff), SrcCommunities)"),
			Expected:    "arrayMap(c -> bitAnd(c, 0xffff), SrcCommunities)",
		}, {
			Description: "no expression",
			Expr:        sb.Expr{},
			Expected:    "",
		}, {
			Description: "hexadecimal number",
			Expr:        sb.Number("0xffff"),
			Expected:    "0xffff",
		}, {
			Description: "lambda",
			Expr: sb.Lambda("c",
				sb.Function("bitAnd", sb.Column("c"), sb.Number("0xffff"))),
			Expected: "c -> bitAnd(c, 0xffff)",
		}, {
			Description: "not between",
			Expr: sb.NotBetween(sb.Column("SrcAddr"),
				sb.Function("toIPv6", sb.String("::1")),
				sb.Function("toIPv6", sb.String("::2"))),
			Expected: `SrcAddr NOT BETWEEN toIPv6('::1') AND toIPv6('::2')`,
		}, {
			Description: "in a subquery",
			Expr: sb.Op(sb.Column("SrcAS"), "IN",
				sb.Select(sb.Column("asn")).From(sb.Table("asns")).Subquery()),
			Expected: "SrcAS IN (SELECT asn FROM asns)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if diff := helpers.Diff(tc.Expr.String(), tc.Expected); diff != "" {
				t.Errorf("String() (-got, +want):\n%s", diff)
			}
		})
	}
}

// TestOpPrecedence checks an operand binding less tightly than the operator
// above it gets parentheses, and that an operand which does not need them keeps
// none. Without this, the SQL would not say what the tree says.
func TestOpPrecedence(t *testing.T) {
	a, b, c := sb.Column("a"), sb.Column("b"), sb.Column("c")
	cases := []struct {
		Description string
		Expr        sb.Expr
		Expected    string
	}{
		{
			Description: "addition below a multiplication",
			Expr:        sb.Op(sb.Op(a, "+", b), "*", c),
			Expected:    "(a + b) * c",
		}, {
			Description: "addition below a cast",
			Expr:        sb.Cast(sb.Op(a, "+", b), "UInt128"),
			Expected:    "(a + b)::UInt128",
		}, {
			// Subtraction is not associative, so the tree has to be kept.
			Description: "subtraction on the right of a subtraction",
			Expr:        sb.Op(a, "-", sb.Op(b, "-", c)),
			Expected:    "a - (b - c)",
		}, {
			Description: "or below an and",
			Expr:        sb.Op(sb.Or(a, b), "AND", c),
			Expected:    "(a OR b) AND c",
		}, {
			// The operators associate to the left, so this reads as written.
			Description: "subtraction then addition",
			Expr:        sb.Op(sb.Op(a, "-", b), "+", c),
			Expected:    "a - b + c",
		}, {
			Description: "multiplication then division",
			Expr:        sb.Op(sb.Op(a, "*", b), "/", c),
			Expected:    "a * b / c",
		}, {
			// AND is associative, so no parentheses are needed. A filter is
			// combined with the time range this way.
			Description: "and on the right of an and",
			Expr:        sb.Op(a, "AND", sb.Op(b, "AND", c)),
			Expected:    "a AND b AND c",
		}, {
			Description: "or on the right of an or",
			Expr:        sb.Op(a, "OR", sb.Op(b, "OR", c)),
			Expected:    "a OR b OR c",
		}, {
			Description: "comparison below an and",
			Expr:        sb.Op(sb.Op(a, "=", b), "AND", c),
			Expected:    "a = b AND c",
		}, {
			Description: "addition below a comparison",
			Expr:        sb.Op(a, "=", sb.Op(b, "+", c)),
			Expected:    "a = b + c",
		}, {
			// NOT binds tighter than AND.
			Description: "negation below an and",
			Expr:        sb.Op(sb.Not(sb.Op(a, "=", b)), "AND", c),
			Expected:    "NOT (a = b) AND c",
		}, {
			Description: "negation below a comparison",
			Expr:        sb.Op(sb.Not(a), "=", b),
			Expected:    "(NOT a) = b",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if diff := helpers.Diff(tc.Expr.String(), tc.Expected); diff != "" {
				t.Errorf("String() (-got, +want):\n%s", diff)
			}
		})
	}
}

// TestIdentifierQuoting checks a name needing backticks gets them. Column names
// come from the schema and from the custom dictionaries, so they are not all
// bare identifiers. It also checks what is not a name keeps its own form.
func TestIdentifierQuoting(t *testing.T) {
	cases := []struct {
		Description string
		Expr        sb.Expr
		Expected    string
	}{
		{
			Description: "column with a space",
			Expr:        sb.Column("Src Addr"),
			Expected:    "`Src Addr`",
		}, {
			Description: "column with a backtick",
			Expr:        sb.Column("Src`Addr"),
			Expected:    "`Src``Addr`",
		}, {
			Description: "column starting with a digit",
			Expr:        sb.Column("1stAS"),
			Expected:    "`1stAS`",
		}, {
			Description: "alias with a space",
			Expr:        sb.Alias(sb.Column("SrcAS"), "my alias"),
			Expected:    "SrcAS AS `my alias`",
		}, {
			// A type is not an identifier, it keeps its parentheses.
			Description: "cast to a parameterized type",
			Expr:        sb.Cast(sb.Column("SrcAS"), "Array(UInt64)"),
			Expected:    "SrcAS::Array(UInt64)",
		}, {
			// The star is not a column named "*".
			Description: "star",
			Expr:        sb.Star(),
			Expected:    "*",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if diff := helpers.Diff(tc.Expr.String(), tc.Expected); diff != "" {
				t.Errorf("String() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	source := sb.Select(sb.Star()).
		From(sb.Table("flows")).
		Setting("asterisk_include_alias_columns", sb.Uint(1))
	rows := sb.Select(sb.Column("ExporterName")).
		From(sb.Table("source")).
		GroupBy(sb.Column("ExporterName")).
		OrderBy(sb.Order(sb.MustParseExpr("SUM(Bytes)")).Desc()).
		Limit(10)
	inner := sb.Select(
		sb.Alias(sb.Column("TimeReceived"), "time"),
		sb.Alias(sb.MustParseExpr("SUM(Bytes)/60"), "xps"),
	).
		From(sb.Table("source")).
		Where(sb.Between(sb.Column("TimeReceived"),
			sb.Function("toDateTime", sb.String("2022-04-10 15:45:00")),
			sb.Function("toDateTime", sb.String("2022-04-11 15:45:00")))).
		GroupBy(sb.Column("time")).
		OrderBy(sb.Order(sb.Column("time")).Fill(
			sb.Function("toDateTime", sb.String("2022-04-10 15:45:00")),
			sb.Function("toDateTime", sb.String("2022-04-11 15:45:00")),
			sb.Uint(60))).
		Interpolate("dimensions", sb.Function("emptyArrayString"))
	got := sb.Select(sb.Alias(sb.Uint(1), "axis"), sb.Star()).
		With("source", source).
		With("rows", rows).
		FromSelect(inner).
		String()
	expected := `WITH
  source AS (SELECT
    *
  FROM
    flows
  SETTINGS
    asterisk_include_alias_columns=1),
  rows AS (SELECT
    ExporterName
  FROM
    source
  GROUP BY
    ExporterName
  ORDER BY
    SUM(Bytes) DESC
  LIMIT 10)
SELECT
  1 AS axis,
  *
FROM
  (SELECT
    TimeReceived AS time,
    SUM(Bytes) / 60 AS xps
  FROM
    source
  WHERE
    TimeReceived BETWEEN toDateTime('2022-04-10 15:45:00') AND toDateTime('2022-04-11 15:45:00')
  GROUP BY
    time
  ORDER BY
    time WITH FILL FROM toDateTime('2022-04-10 15:45:00') TO toDateTime('2022-04-11 15:45:00') STEP 60
    INTERPOLATE (dimensions AS emptyArrayString()))`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

// TestSelectEmptyClauses checks a clause is dropped when it gets nothing to
// work with, so a caller does not have to test for that itself.
func TestSelectEmptyClauses(t *testing.T) {
	got := sb.Select(sb.Star()).
		From(sb.Table("flows")).
		Where(sb.Op(sb.Column("SrcAS"), "=", sb.Uint(12322))).
		GroupBy(sb.Columns("SrcAS", "DstAS")...).
		OrderBy(sb.Order(sb.Column("SrcAS"))).
		// Building the same query without a filter, a grouping or a sort must
		// remove the clauses again.
		Where(sb.Expr{}).
		GroupBy().
		OrderBy().
		String()
	expected := `SELECT
  *
FROM
  flows`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestSelectStarReplace(t *testing.T) {
	got := sb.Select().
		Item(sb.Star(), sb.Replace(
			sb.Alias(sb.Function("tupleElement",
				sb.Function("IPv6CIDRToRange",
					sb.Column("SrcAddr"), sb.Uint(48)),
				sb.Uint(1)), "SrcAddr"))).
		From(sb.Table("flows")).
		String()
	expected := `SELECT
  * REPLACE(tupleElement(IPv6CIDRToRange(SrcAddr, 48), 1) AS SrcAddr)
FROM
  flows`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestSelectStarExcept(t *testing.T) {
	got := sb.Select().
		Item(sb.Star(), sb.Except("SrcCommunities", "DstCommunities")).
		From(sb.Table("flows")).
		String()
	expected := `SELECT
  * EXCEPT(SrcCommunities, DstCommunities)
FROM
  flows`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestSelectUnionAll(t *testing.T) {
	first := sb.Select(sb.Alias(sb.Uint(1), "axis")).From(sb.Table("flows"))
	second := sb.Select(sb.Alias(sb.Uint(2), "axis")).From(sb.Table("flows"))
	third := sb.Select(sb.Alias(sb.Uint(3), "axis")).From(sb.Table("flows"))
	got := first.UnionAll(second).UnionAll(third).String()
	expected := `SELECT
  1 AS axis
FROM
  flows
UNION ALL
SELECT
  2 AS axis
FROM
  flows
UNION ALL
SELECT
  3 AS axis
FROM
  flows`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestSelectUnionDistinct(t *testing.T) {
	first := sb.Select(sb.Column("port")).From(sb.Table("tcp"))
	second := sb.Select(sb.Column("port")).From(sb.Table("udp"))
	got := first.UnionDistinct(second).String()
	expected := `SELECT
  port
FROM
  tcp
UNION DISTINCT
SELECT
  port
FROM
  udp`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestSelectDistinct(t *testing.T) {
	got := sb.Select(sb.Column("ExporterName")).
		Distinct().
		From(sb.Table("flows")).
		String()
	expected := `SELECT DISTINCT
  ExporterName
FROM
  flows`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestSelectScalarCTE(t *testing.T) {
	period := sb.Select(sb.MustParseExpr("MAX(TimeReceived) - MIN(TimeReceived)")).From(sb.Table("source"))
	got := sb.Select(sb.Alias(sb.MustParseExpr("SUM(Bytes)/range"), "xps")).
		WithScalar(period, "range").
		From(sb.Table("source")).
		String()
	expected := `WITH
  (SELECT
    MAX(TimeReceived) - MIN(TimeReceived)
  FROM
    source) AS range
SELECT
  SUM(Bytes) / range AS xps
FROM
  source`
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Errorf("String() (-got, +want):\n%s", diff)
	}
}

func TestParseExprErrors(t *testing.T) {
	cases := []string{
		"SrcAddr, DstAddr",    // two expressions
		"1 +",                 // incomplete
		"toIPv6('::1'",        // unbalanced parenthesis
		"SrcAS = 1; SELECT 2", // two statements
		// The expression is parsed inside a SELECT, so anything written after
		// it would be taken for a clause of that SELECT and dropped.
		"SrcAS = 1 LIMIT 1",
		"SrcAS = 1 FROM flows",
		"SrcAS = 1 WHERE DstAS = 2",
		"SrcAS = 1 SETTINGS max_threads = 1",
		"SrcAS = 1 UNION ALL SELECT 1",
		"SrcAS = 1 ORDER BY SrcAS",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if _, err := sb.ParseExpr(tc); err == nil {
				t.Errorf("ParseExpr(%q) did not return an error", tc)
			}
		})
	}
}

func TestMustParseExprPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParseExpr() did not panic on an invalid expression")
		}
	}()
	sb.MustParseExpr("1 +")
}

// TestNormalize checks two queries written differently compare equal once
// normalized, while two different queries do not.
func TestNormalize(t *testing.T) {
	compact := `SELECT a, b FROM flows WHERE a=1 AND b=2 ORDER BY a DESC`
	spread := `SELECT
  a,
  b
FROM flows
WHERE a = 1
  AND b = 2
ORDER BY a DESC`
	if diff := helpers.Diff(sb.Normalize(t, compact), sb.Normalize(t, spread)); diff != "" {
		t.Errorf("Normalize() (-compact, +spread):\n%s", diff)
	}
	if sb.Normalize(t, compact) == sb.Normalize(t, `SELECT a, b FROM flows WHERE a=1 OR b=2 ORDER BY a DESC`) {
		t.Error("Normalize() made two different queries equal")
	}
	got := sb.NormalizeAll(t, []string{compact, spread})
	if diff := helpers.Diff(got[0], got[1]); diff != "" {
		t.Errorf("NormalizeAll() (-got, +want):\n%s", diff)
	}
}

func TestSQLMatcher(t *testing.T) {
	matcher := sb.SQLMatcher(t, `SELECT a FROM flows WHERE a=1`)
	cases := []struct {
		Description string
		Input       any
		Expected    bool
	}{
		{"same query, other layout", "SELECT\n  a\nFROM\n  flows\nWHERE\n  a = 1", true},
		{"another query", "SELECT a FROM flows WHERE a = 2", false},
		{"not SQL", "this is not SQL", false},
		{"not a string", 42, false},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if diff := helpers.Diff(matcher.Matches(tc.Input), tc.Expected); diff != "" {
				t.Errorf("Matches(%v) (-got, +want):\n%s", tc.Input, diff)
			}
		})
	}
	if matcher.String() == "" {
		t.Error("String() is empty")
	}
}

func TestCheckStatement(t *testing.T) {
	sb.CheckStatement(t, `SELECT a FROM flows WHERE a = 1`)
}
