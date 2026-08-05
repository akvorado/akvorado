// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder_test

import (
	"testing"

	"akvorado/common/helpers"
	sb "akvorado/common/sqlbuilder"
)

func TestTableName(t *testing.T) {
	cases := []struct {
		Description string
		Table       sb.TableName
		Expected    string
	}{
		{
			Description: "bare name",
			Table:       sb.Table("flows"),
			Expected:    "flows",
		}, {
			Description: "with a database",
			Table:       sb.Table("flows").In("akvorado"),
			Expected:    "akvorado.flows",
		}, {
			Description: "name needing quotes",
			Table:       sb.Table("flows-1").In("my db"),
			Expected:    "`my db`.`flows-1`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if diff := helpers.Diff(tc.Table.String(), tc.Expected); diff != "" {
				t.Errorf("String() (-got, +want):\n%s", diff)
			}
		})
	}
}

func TestCreateTable(t *testing.T) {
	columns, err := sb.ParseColumnDefs(
		"`TimeReceived` DateTime CODEC(DoubleDelta, LZ4),\n" +
			"`ExporterAddress` LowCardinality(IPv6),\n" +
			"`SrcNetPrefix` String ALIAS IPv6CIDRToRange(SrcAddr, SrcNetMask).1::String")
	if err != nil {
		t.Fatalf("ParseColumnDefs() error:\n%+v", err)
	}
	got := sb.CreateTable(sb.Table("flows").In("akvorado")).
		Columns(columns...).
		Engine(sb.NewEngine("SummingMergeTree", sb.Tuple(sb.Columns("Bytes", "Packets")...))).
		PartitionBy(sb.Function("toYYYYMMDDhhmmss",
			sb.Function("toStartOfInterval", sb.Column("TimeReceived"),
				sb.Interval(sb.Uint(86400), "second")))).
		PrimaryKey(sb.Function("toStartOfFiveMinutes", sb.Column("TimeReceived"))).
		OrderBy(sb.Columns("TimeReceived", "ExporterAddress")...).
		TTL(sb.Op(sb.Column("TimeReceived"), "+", sb.Function("toIntervalDay", sb.Uint(15)))).
		Setting("index_granularity", sb.Uint(8192)).
		String()
	expected := `CREATE TABLE akvorado.flows
(
  ` + "`TimeReceived`" + ` DateTime CODEC(DoubleDelta, LZ4),
  ` + "`ExporterAddress`" + ` LowCardinality(IPv6),
  ` + "`SrcNetPrefix`" + ` String ALIAS IPv6CIDRToRange(SrcAddr, SrcNetMask).1::String
)
ENGINE = SummingMergeTree((Bytes, Packets))
ORDER BY (TimeReceived, ExporterAddress)
PARTITION BY toYYYYMMDDhhmmss(toStartOfInterval(TimeReceived, INTERVAL 86400 second))
PRIMARY KEY toStartOfFiveMinutes(TimeReceived)
TTL TimeReceived + toIntervalDay(15)
SETTINGS index_granularity=8192`
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("CreateTable() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestCreateTableOrReplace(t *testing.T) {
	got := sb.CreateTable(sb.Table("flows")).
		Columns(sb.NewColumnDef("TimeReceived", "DateTime")).
		Engine(sb.NewEngine("Null")).
		OrReplace().
		String()
	expected := "CREATE OR REPLACE TABLE flows (`TimeReceived` DateTime) ENGINE = Null"
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("CreateTable() (-got, +want):\n%s", diff)
	}
}

func TestCreateMaterializedView(t *testing.T) {
	query := sb.Select(
		sb.Column("TimeReceived"),
		sb.Alias(sb.Index(sb.Array(sb.Column("InIfName"), sb.Column("OutIfName")),
			sb.Column("num")), "IfName")).
		From(sb.Table("flows").In("akvorado")).
		ArrayJoin(sb.Alias(sb.Function("arrayEnumerate",
			sb.Array(sb.Uint(1), sb.Uint(2))), "num"))
	view := sb.CreateMaterializedView(
		sb.Table("exporters_consumer"), sb.Table("exporters"), query)
	expected := `CREATE MATERIALIZED VIEW exporters_consumer TO exporters
AS SELECT TimeReceived, [InIfName, OutIfName][num] AS IfName
FROM akvorado.flows ARRAY JOIN arrayEnumerate([1, 2]) AS num`
	if diff := helpers.Diff(view.String(), sb.Normalize(t, expected)); diff != "" {
		t.Errorf("CreateMaterializedView() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, view.String())
}

func TestCreateDictionary(t *testing.T) {
	got := sb.CreateDictionary(sb.Table("asns").In("akvorado")).
		Attributes(
			sb.Attribute("asn", "UInt32").Injective(),
			sb.Attribute("name", "String").Default("None")).
		PrimaryKey(sb.Column("asn")).
		Source("HTTP",
			sb.Param("URL", sb.String("http://127.0.0.1:8080/asns.csv")),
			sb.Param("FORMAT", sb.String("CSVWithNames")),
			sb.ParamGroup("CREDENTIALS",
				sb.Param("user", sb.String("alfred")),
				sb.Param("password", sb.String("it's a secret")))).
		Lifetime(0, 3600).
		Layout("HASHED").
		Setting("format_csv_allow_single_quotes", sb.Uint(0)).
		String()
	expected := `CREATE DICTIONARY akvorado.asns (asn UInt32 INJECTIVE, name String DEFAULT 'None')
PRIMARY KEY asn
SOURCE(HTTP(URL 'http://127.0.0.1:8080/asns.csv' FORMAT 'CSVWithNames' CREDENTIALS(user 'alfred' password 'it\'s a secret')))
LIFETIME(MIN 0 MAX 3600)
LAYOUT(HASHED())
SETTINGS(format_csv_allow_single_quotes = 0)`
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("CreateDictionary() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestAlterTable(t *testing.T) {
	added, err := sb.ParseColumnDef("`SrcVlan` UInt16")
	if err != nil {
		t.Fatalf("ParseColumnDef() error:\n%+v", err)
	}
	modified, err := sb.ParseColumnDef("`SrcAS` UInt32 CODEC(ZSTD(1))")
	if err != nil {
		t.Fatalf("ParseColumnDef() error:\n%+v", err)
	}
	alter := sb.AlterTable(sb.Table("flows"))
	if diff := helpers.Diff(alter.Len(), 0); diff != "" {
		t.Errorf("Len() (-got, +want):\n%s", diff)
	}
	got := alter.
		AddColumn(added, "SrcAddr").
		ModifyColumn(modified).
		DropColumn("DstVlan").
		AddIndex("idx_srcas", sb.Column("SrcAS"), sb.MustParseExpr("set(100)"), 4).
		DropIndex("idx_dstas").
		ModifyOrderBy(sb.Columns("TimeReceived", "ExporterAddress")...).
		String()
	expected := "ALTER TABLE flows ADD COLUMN `SrcVlan` UInt16 AFTER SrcAddr, " +
		"MODIFY COLUMN `SrcAS` UInt32 CODEC(ZSTD(1)), " +
		"DROP COLUMN DstVlan, " +
		"ADD INDEX idx_srcas SrcAS TYPE set(100) GRANULARITY 4, " +
		"DROP INDEX idx_dstas, " +
		"MODIFY ORDER BY (TimeReceived, ExporterAddress)"
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("AlterTable() (-got, +want):\n%s", diff)
	}
	if diff := helpers.Diff(alter.Len(), 6); diff != "" {
		t.Errorf("Len() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestAlterTableSettings(t *testing.T) {
	// ClickHouse takes MODIFY SETTING only once, with every setting in it.
	got := sb.AlterTable(sb.Table("flows")).
		ModifySetting("index_granularity", sb.Uint(8192)).
		ModifySetting("ttl_only_drop_parts", sb.Uint(1)).
		String()
	expected := "ALTER TABLE flows MODIFY SETTING index_granularity = 8192, ttl_only_drop_parts = 1"
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("AlterTable() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestAlterTableTTL(t *testing.T) {
	got := sb.AlterTable(sb.Table("flows")).
		ModifyTTL(sb.Op(sb.Column("TimeReceived"), "+",
			sb.Function("toIntervalSecond", sb.Uint(3600)))).
		String()
	expected := "ALTER TABLE flows MODIFY TTL TimeReceived + toIntervalSecond(3600)"
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("AlterTable() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestAlterTableAddFirstColumn(t *testing.T) {
	def, err := sb.ParseColumnDef("`TimeReceived` DateTime")
	if err != nil {
		t.Fatalf("ParseColumnDef() error:\n%+v", err)
	}
	got := sb.AlterTable(sb.Table("flows")).AddColumn(def, "").String()
	expected := "ALTER TABLE flows ADD COLUMN `TimeReceived` DateTime"
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("AlterTable() (-got, +want):\n%s", diff)
	}
}

func TestDropTable(t *testing.T) {
	got := sb.DropTable(sb.Table("flows_consumer")).String()
	if diff := helpers.Diff(got, "DROP TABLE IF EXISTS flows_consumer SYNC"); diff != "" {
		t.Errorf("DropTable() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestParseColumnDefsErrors(t *testing.T) {
	cases := []string{
		"SELECT 1",
		"`SrcAddr` IPv6(",
		"INDEX idx_srcas SrcAS TYPE set(100) GRANULARITY 4",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if _, err := sb.ParseColumnDefs(tc); err == nil {
				t.Errorf("ParseColumnDefs(%q) did not error", tc)
			}
		})
	}
	if _, err := sb.ParseColumnDef("`a` UInt8, `b` UInt8"); err == nil {
		t.Error("ParseColumnDef() with two columns did not error")
	}
}

func TestParseExprs(t *testing.T) {
	exprs, err := sb.ParseExprs([]string{"SrcAS", "c_DstASPath AS DstASPath"})
	if err != nil {
		t.Fatalf("ParseExprs() error:\n%+v", err)
	}
	got := []string{exprs[0].String(), exprs[1].String()}
	if diff := helpers.Diff(got, []string{"SrcAS", "c_DstASPath AS DstASPath"}); diff != "" {
		t.Errorf("ParseExprs() (-got, +want):\n%s", diff)
	}
	if _, err := sb.ParseExprs([]string{"SrcAS", "toIPv6('::1'"}); err == nil {
		t.Error("ParseExprs() did not error")
	}
}

func TestMatches(t *testing.T) {
	create := sb.CreateTable(sb.Table("flows").In("akvorado")).
		Columns(sb.NewColumnDef("TimeReceived", "DateTime")).
		Engine(sb.NewEngine("MergeTree")).
		OrderBy(sb.Column("TimeReceived"))
	cases := []struct {
		Description string
		SQL         string
		Expected    bool
	}{
		{
			Description: "same statement, another layout and quoting",
			SQL:         "CREATE TABLE `akvorado`.`flows` (`TimeReceived` DateTime)\nENGINE = MergeTree\nORDER BY TimeReceived",
			Expected:    true,
		}, {
			Description: "clauses in the order ClickHouse writes them",
			SQL:         "CREATE TABLE akvorado.flows (`TimeReceived` DateTime) ENGINE = MergeTree ORDER BY TimeReceived",
			Expected:    true,
		}, {
			Description: "another column type",
			SQL:         "CREATE TABLE akvorado.flows (`TimeReceived` DateTime64) ENGINE = MergeTree ORDER BY TimeReceived",
			Expected:    false,
		}, {
			Description: "another database",
			SQL:         "CREATE TABLE other.flows (`TimeReceived` DateTime) ENGINE = MergeTree ORDER BY TimeReceived",
			Expected:    false,
		}, {
			Description: "does not parse",
			SQL:         "CREATE TABLE akvorado.flows (`TimeReceived` DateTime) ENGINE",
			Expected:    false,
		}, {
			Description: "empty",
			SQL:         "",
			Expected:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			if diff := helpers.Diff(create.Matches(tc.SQL), tc.Expected); diff != "" {
				t.Errorf("Matches(%q) (-got, +want):\n%s", tc.SQL, diff)
			}
		})
	}
}

func TestMatchesQuery(t *testing.T) {
	query := sb.Select(sb.Column("SrcAS")).From(sb.Table("flows").In("akvorado"))
	if !query.Matches("SELECT SrcAS\nFROM `akvorado`.`flows`") {
		t.Error("Matches() did not match the same query")
	}
	if query.Matches("SELECT DstAS FROM akvorado.flows") {
		t.Error("Matches() matched another query")
	}
}

// TestMatchesAliasColumn checks the shape ClickHouse writes back for the flows
// table: a "::" cast in an ALIAS comes back as a CAST call.
func TestMatchesAliasColumn(t *testing.T) {
	columns, err := sb.ParseColumnDefs(
		"`SrcNetPrefix` String ALIAS IPv6CIDRToRange(SrcAddr, (96 + SrcNetMask)::UInt8).1::String")
	if err != nil {
		t.Fatalf("ParseColumnDefs() error:\n%+v", err)
	}
	create := sb.CreateTable(sb.Table("flows")).Columns(columns...)
	if create.Matches("CREATE TABLE flows (`SrcNetPrefix` String ALIAS CAST((IPv6CIDRToRange(SrcAddr, CAST(96 + SrcNetMask, 'UInt8'))).1, 'String'))") {
		t.Error("Matches() matched a different alias")
	}
	if !create.Matches("CREATE TABLE flows (`SrcNetPrefix` String ALIAS IPv6CIDRToRange(SrcAddr, (96 + SrcNetMask)::UInt8).1::String)") {
		t.Error("Matches() did not match the same alias")
	}
}

func TestCreateDistributedTable(t *testing.T) {
	got := sb.CreateTable(sb.Table("flows").In("akvorado")).
		Columns(sb.NewColumnDef("TimeReceived", "DateTime")).
		Engine(sb.NewEngine("Distributed",
			sb.String("cluster"), sb.String("akvorado"), sb.String("flows_local"),
			sb.Function("rand"))).
		String()
	expected := "CREATE TABLE akvorado.flows (`TimeReceived` DateTime)\n" +
		"ENGINE = Distributed('cluster', 'akvorado', 'flows_local', rand())"
	if diff := helpers.Diff(got, sb.Normalize(t, expected)); diff != "" {
		t.Errorf("CreateTable() (-got, +want):\n%s", diff)
	}
	sb.CheckStatement(t, got)
}

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct {
		Pos      helpers.Pos
		Input    string
		Expected string
	}{
		{helpers.Mark(), "akvorado", "akvorado"},
		{helpers.Mark(), "simple_name", "simple_name"},
		{helpers.Mark(), "_leading", "_leading"},
		{helpers.Mark(), "my`cluster", "`my``cluster`"},
		{helpers.Mark(), "with spaces", "`with spaces`"},
		{helpers.Mark(), "with-dash", "`with-dash`"},
		{helpers.Mark(), "123start", "`123start`"},
		{helpers.Mark(), "", "``"},
	}
	for _, tc := range cases {
		got := sb.QuoteIdentifier(tc.Input)
		if diff := helpers.Diff(got, tc.Expected); diff != "" {
			t.Errorf("%sQuoteIdentifier(%q) (-got, +want):\n%s", tc.Pos, tc.Input, diff)
		}
		// A table name is written the same way.
		if diff := helpers.Diff(sb.Table(tc.Input).String(), tc.Expected); diff != "" {
			t.Errorf("%sTable(%q).String() (-got, +want):\n%s", tc.Pos, tc.Input, diff)
		}
	}
}
