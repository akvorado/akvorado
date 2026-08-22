// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// TableName is the name of a table, a view or a dictionary.
type TableName struct {
	database string
	name     string
}

// Table names a table in the current database.
func Table(name string) TableName {
	return TableName{name: name}
}

// In sets the database holding the table.
func (t TableName) In(database string) TableName {
	t.database = database
	return t
}

// String returns the name as written in SQL.
func (t TableName) String() string {
	return parser.Format(t.node())
}

func (t TableName) node() *parser.TableIdentifier {
	identifier := &parser.TableIdentifier{Table: ident(t.name)}
	if t.database != "" {
		identifier.Database = ident(t.database)
	}
	return identifier
}

// Statement is a complete SQL statement, whatever it was built from.
type Statement interface {
	// String renders the statement as SQL.
	String() string
	// OnCluster adds an ON CLUSTER clause to the statement. The statement it
	// is called on is modified, not copied.
	OnCluster(cluster string) Statement
}

// statement is embedded in the statement builders below. It carries how a
// statement is rendered and compared.
type statement struct {
	node parser.Expr
}

// String renders the statement as indented, multi-line SQL.
func (s statement) String() string {
	formatter := parser.NewFormatter().WithBeautify()
	formatter.WriteExpr(s.node)
	return formatter.String()
}

// Matches tells if the provided SQL means the same as the statement. Only the
// meaning is compared: layout and identifier quoting do not matter. SQL that
// cannot be parsed never matches.
func (s statement) Matches(sql string) bool {
	mine, ok := canonical(parser.Format(s.node))
	if !ok {
		return false
	}
	theirs, ok := canonical(sql)
	return ok && mine == theirs
}

// OnCluster adds an ON CLUSTER clause to the statement, so ClickHouse runs it on
// every server of the cluster. The statement is modified, not copied. A
// statement not accepting the clause, like SELECT, is left as is.
func (s statement) OnCluster(cluster string) Statement {
	withCluster(s.node, cluster)
	return s
}

// withCluster sets the ON CLUSTER clause of a node. The node is modified. A
// node without such a field, like SELECT, is left as is.
func withCluster(node parser.Expr, cluster string) {
	const clusterField = "OnCluster"

	// The SYSTEM statements carry the clause on the node below.
	if system, ok := node.(*parser.SystemStmt); ok {
		withCluster(system.Expr, cluster)
		return
	}

	// Look for the cluster field for any other node.
	value := reflect.ValueOf(node)
	if value.Kind() != reflect.Pointer || value.Elem().Kind() != reflect.Struct {
		return
	}
	field := value.Elem().FieldByName(clusterField)
	if !field.IsValid() || field.Type() != reflect.TypeFor[*parser.ClusterClause]() {
		return
	}

	field.Set(reflect.ValueOf(&parser.ClusterClause{Expr: ident(cluster)}))
}

// rawType is a ClickHouse type written as is. Types come from the schema as
// text, they are not built here. They are not identifiers either: something
// like "Array(UInt64)" would not survive quoting.
type rawType string

func (t rawType) Pos() parser.Pos {
	return 0
}
func (t rawType) End() parser.Pos {
	return 0
}
func (t rawType) Type() string {
	return string(t)
}
func (t rawType) Accept(_ parser.ASTVisitor) error {
	return nil
}
func (t rawType) FormatSQL(formatter *parser.Formatter) {
	formatter.WriteString(string(t))
}

// ColumnDef is the definition of a column, as written between the parentheses
// of a CREATE TABLE statement.
type ColumnDef struct {
	node *parser.ColumnDef
}

// NewColumnDef builds the definition of a column of the provided ClickHouse
// type. The name is always quoted, like ClickHouse writes it back and like the
// definitions coming from the schema.
func NewColumnDef(name, typeName string) ColumnDef {
	return ColumnDef{node: &parser.ColumnDef{
		Name: &parser.NestedIdentifier{Ident: backquotedIdent(name)},
		Type: rawType(typeName),
	}}
}

// ParseColumnDefs parses a comma-separated list of column definitions, for
// example "`SrcAddr` IPv6 CODEC(ZSTD(1)), `SrcNetMask` UInt8". Definitions come
// from the schema as text, so they are parsed instead of being built.
func ParseColumnDefs(sql string) ([]ColumnDef, error) {
	statements, err := parse(fmt.Sprintf("CREATE TABLE t (%s)", sql))
	if err != nil {
		return nil, err
	}
	if len(statements) != 1 {
		return nil, fmt.Errorf("expected one list of columns, got %d", len(statements))
	}
	create, ok := statements[0].(*parser.CreateTable)
	if !ok || create.TableSchema == nil {
		return nil, errors.New("not a list of columns")
	}
	defs := make([]ColumnDef, 0, len(create.TableSchema.Columns))
	for _, column := range create.TableSchema.Columns {
		def, ok := column.(*parser.ColumnDef)
		if !ok {
			return nil, fmt.Errorf("%q is not a column definition", parser.Format(column))
		}
		defs = append(defs, ColumnDef{node: def})
	}
	return defs, nil
}

// ParseColumnDef parses a single column definition.
func ParseColumnDef(sql string) (ColumnDef, error) {
	defs, err := ParseColumnDefs(sql)
	if err != nil {
		return ColumnDef{}, err
	}
	if len(defs) != 1 {
		return ColumnDef{}, fmt.Errorf("expected one column, got %d", len(defs))
	}
	return defs[0], nil
}

// Engine is the engine of a table, with its parameters, TTL and settings.
type Engine struct {
	node *parser.EngineExpr
}

// NewEngine builds a table engine.
func NewEngine(name string, args ...Expr) Engine {
	node := &parser.EngineExpr{Name: name}
	if len(args) > 0 {
		node.Params = &parser.ParamExprList{
			Items: &parser.ColumnExprList{Items: nodes(args)},
		}
	}
	return Engine{node: node}
}

// ParseEngine parses a table engine as given by ClickHouse in engine_full
// column of system.tables.
func ParseEngine(sql string) (Engine, error) {
	statements, err := parse(fmt.Sprintf("CREATE TABLE t (x UInt8) ENGINE = %s", sql))
	if err != nil {
		return Engine{}, err
	}
	if len(statements) != 1 {
		return Engine{}, fmt.Errorf("expected one statement, got %d", len(statements))
	}
	create, ok := statements[0].(*parser.CreateTable)
	if !ok || create.Engine == nil {
		return Engine{}, errors.New("not a table engine")
	}
	return Engine{node: create.Engine}, nil
}

// Name returns the name of the engine.
func (e Engine) Name() string {
	if e.node == nil {
		return ""
	}
	return e.node.Name
}

// Args returns the arguments the engine was given.
func (e Engine) Args() []Expr {
	if e.node == nil || e.node.Params == nil || e.node.Params.Items == nil {
		return nil
	}
	items := e.node.Params.Items.Items
	args := make([]Expr, len(items))
	for i, item := range items {
		// Each argument comes wrapped in a column expression, as the parser
		// reads them like a select list.
		if column, ok := item.(*parser.ColumnExpr); ok && column.Alias == nil {
			item = column.Expr
		}
		args[i] = wrap(item)
	}
	return args
}

// TTL returns the TTL of the table.
func (e Engine) TTL() Expr {
	if e.node == nil || e.node.TTL == nil || len(e.node.TTL.Items) != 1 {
		return Expr{}
	}
	return wrap(e.node.TTL.Items[0].Expr)
}

// Settings returns the settings of the table, indexed by name.
func (e Engine) Settings() map[string]Expr {
	settings := map[string]Expr{}
	if e.node == nil || e.node.Settings == nil {
		return settings
	}
	for _, item := range e.node.Settings.Items {
		settings[item.Name.Name] = wrap(item.Expr)
	}
	return settings
}

// ReplicatedPath returns the path of the table in ZooKeeper. This is the first
// argument of a Replicated*MergeTree engine. False is returned for a table that
// is not replicated or that gets its path from somewhere else than a literal.
func (e Engine) ReplicatedPath() (string, bool) {
	name := e.Name()
	if !strings.HasPrefix(name, "Replicated") || !strings.HasSuffix(name, "MergeTree") {
		return "", false
	}
	args := e.Args()
	if len(args) == 0 {
		return "", false
	}
	literal, ok := args[0].node.(*parser.StringLiteral)
	if !ok {
		return "", false
	}
	return stringUnescaper.Replace(literal.Literal), true
}

// keyExpr turns a list of keys into what follows ORDER BY or PRIMARY KEY. A
// single key needs no parentheses, this is also how ClickHouse writes it back.
func keyExpr(exprs []Expr) parser.Expr {
	if len(exprs) == 1 {
		return exprs[0].node
	}
	return Tuple(exprs...).node
}

// CreateTableStatement builds a CREATE TABLE statement.
type CreateTableStatement struct {
	statement
	create *parser.CreateTable
}

// CreateTable starts a CREATE TABLE statement.
func CreateTable(name TableName) *CreateTableStatement {
	create := &parser.CreateTable{Name: name.node()}
	return &CreateTableStatement{statement{node: create}, create}
}

// OrReplace replaces the table when it already exists.
func (s *CreateTableStatement) OrReplace() *CreateTableStatement {
	s.create.OrReplace = true
	return s
}

// Columns sets the columns of the table.
func (s *CreateTableStatement) Columns(defs ...ColumnDef) *CreateTableStatement {
	columns := make([]parser.Expr, len(defs))
	for i, def := range defs {
		columns[i] = def.node
	}
	s.create.TableSchema = &parser.TableSchemaClause{Columns: columns}
	return s
}

// engine returns the engine clause, which also carries the clauses below it.
func (s *CreateTableStatement) engine() *parser.EngineExpr {
	if s.create.Engine == nil {
		s.create.Engine = &parser.EngineExpr{}
	}
	return s.create.Engine
}

// Engine sets the engine of the table.
func (s *CreateTableStatement) Engine(engine Engine) *CreateTableStatement {
	node := s.engine()
	node.Name, node.Params = "", nil
	if engine.node != nil {
		node.Name, node.Params = engine.node.Name, engine.node.Params
	}
	return s
}

// PartitionBy sets the PARTITION BY clause.
func (s *CreateTableStatement) PartitionBy(expr Expr) *CreateTableStatement {
	s.engine().PartitionBy = &parser.PartitionByClause{Expr: expr.node}
	return s
}

// PrimaryKey sets the PRIMARY KEY clause.
func (s *CreateTableStatement) PrimaryKey(exprs ...Expr) *CreateTableStatement {
	s.engine().PrimaryKey = &parser.PrimaryKeyClause{Expr: keyExpr(exprs)}
	return s
}

// OrderBy sets the ORDER BY clause.
func (s *CreateTableStatement) OrderBy(exprs ...Expr) *CreateTableStatement {
	s.engine().OrderBy = &parser.OrderByClause{Items: []parser.Expr{keyExpr(exprs)}}
	return s
}

// TTL sets the TTL clause.
func (s *CreateTableStatement) TTL(expr Expr) *CreateTableStatement {
	s.engine().TTL = &parser.TTLClause{Items: []*parser.TTLExpr{{Expr: expr.node}}}
	return s
}

// Setting adds one entry to the SETTINGS clause.
func (s *CreateTableStatement) Setting(name string, value Expr) *CreateTableStatement {
	node := s.engine()
	if node.Settings == nil {
		node.Settings = &parser.SettingsClause{}
	}
	node.Settings.Items = append(node.Settings.Items,
		&parser.SettingExpr{Name: ident(name), Expr: value.node})
	return s
}

// CreateViewStatement builds a CREATE MATERIALIZED VIEW statement.
type CreateViewStatement struct {
	statement
	view *parser.CreateMaterializedView
}

// CreateMaterializedView starts a CREATE MATERIALIZED VIEW statement feeding
// the destination table with the result of the query.
func CreateMaterializedView(name, destination TableName, query *Query) *CreateViewStatement {
	view := &parser.CreateMaterializedView{
		Name:        name.node(),
		Destination: &parser.DestinationClause{TableIdentifier: destination.node()},
		SubQuery:    &parser.SubQuery{Select: query.query},
	}
	return &CreateViewStatement{statement{node: view}, view}
}

// CreateDictionaryStatement builds a CREATE DICTIONARY statement.
type CreateDictionaryStatement struct {
	statement
	dictionary *parser.CreateDictionary
}

// CreateDictionary starts a CREATE DICTIONARY statement.
func CreateDictionary(name TableName) *CreateDictionaryStatement {
	dictionary := &parser.CreateDictionary{
		Name:   name.node(),
		Schema: &parser.DictionarySchemaClause{},
		Engine: &parser.DictionaryEngineClause{},
	}
	return &CreateDictionaryStatement{statement{node: dictionary}, dictionary}
}

// OrReplace replaces the dictionary when it already exists.
func (s *CreateDictionaryStatement) OrReplace() *CreateDictionaryStatement {
	s.dictionary.OrReplace = true
	return s
}

// Attributes sets the attributes of the dictionary.
func (s *CreateDictionaryStatement) Attributes(attributes ...DictionaryAttribute) *CreateDictionaryStatement {
	s.dictionary.Schema.Attributes = make([]*parser.DictionaryAttribute, len(attributes))
	for i, attribute := range attributes {
		s.dictionary.Schema.Attributes[i] = attribute.node
	}
	return s
}

// PrimaryKey sets the primary key of the dictionary.
func (s *CreateDictionaryStatement) PrimaryKey(keys ...Expr) *CreateDictionaryStatement {
	s.dictionary.Engine.PrimaryKey = &parser.DictionaryPrimaryKeyClause{
		Keys: &parser.ColumnExprList{Items: nodes(keys)},
	}
	return s
}

// Source sets where the dictionary reads its content from.
func (s *CreateDictionaryStatement) Source(name string, params ...SourceParam) *CreateDictionaryStatement {
	s.dictionary.Engine.Source = &parser.DictionarySourceClause{
		Source: ident(name),
		Args:   sourceParams(params),
	}
	return s
}

// Lifetime sets the range, in seconds, for how long the content stays valid.
// ClickHouse picks a random value in it, so the dictionaries do not all reload
// at the same time.
func (s *CreateDictionaryStatement) Lifetime(shortest, longest uint64) *CreateDictionaryStatement {
	s.dictionary.Engine.Lifetime = &parser.DictionaryLifetimeClause{
		Min: &parser.NumberLiteral{Literal: fmt.Sprintf("%d", shortest)},
		Max: &parser.NumberLiteral{Literal: fmt.Sprintf("%d", longest)},
	}
	return s
}

// Layout sets how the content is kept in memory.
func (s *CreateDictionaryStatement) Layout(name string) *CreateDictionaryStatement {
	s.dictionary.Engine.Layout = &parser.DictionaryLayoutClause{Layout: ident(name)}
	return s
}

// Setting adds one entry to the SETTINGS clause.
func (s *CreateDictionaryStatement) Setting(name string, value Expr) *CreateDictionaryStatement {
	if s.dictionary.Engine.Settings == nil {
		s.dictionary.Engine.Settings = &parser.SettingsClause{}
	}
	s.dictionary.Engine.Settings.Items = append(s.dictionary.Engine.Settings.Items,
		&parser.SettingExpr{Name: ident(name), Expr: value.node})
	return s
}

// DictionaryAttribute is one attribute of a dictionary.
type DictionaryAttribute struct {
	node *parser.DictionaryAttribute
}

// Attribute builds an attribute of the provided ClickHouse type.
func Attribute(name, typeName string) DictionaryAttribute {
	return DictionaryAttribute{node: &parser.DictionaryAttribute{
		Name: ident(name),
		Type: rawType(typeName),
	}}
}

// Injective tells that two different keys never share the same value.
func (a DictionaryAttribute) Injective() DictionaryAttribute {
	a.node.Injective = true
	return a
}

// Default sets the value used when the key is not found.
func (a DictionaryAttribute) Default(value string) DictionaryAttribute {
	a.node.Default = &parser.StringLiteral{Literal: stringEscaper.Replace(value)}
	return a
}

// SourceParam is one parameter of the source of a dictionary.
type SourceParam struct {
	node *parser.DictionaryArgExpr
}

// Param builds a source parameter, as in "URL 'http://…'".
func Param(name string, value Expr) SourceParam {
	return SourceParam{node: &parser.DictionaryArgExpr{
		Name:  ident(name),
		Value: value.node,
	}}
}

// ParamGroup builds a source parameter holding other parameters, as in
// "CREDENTIALS(user 'u' password 'p')".
func ParamGroup(name string, params ...SourceParam) SourceParam {
	return SourceParam{node: &parser.DictionaryArgExpr{
		Name: ident(name),
		Args: sourceParams(params),
		// The parentheses are only written when this position is set.
		LParenPos: 1,
	}}
}

func sourceParams(params []SourceParam) []*parser.DictionaryArgExpr {
	args := make([]*parser.DictionaryArgExpr, len(params))
	for i, param := range params {
		args[i] = param.node
	}
	return args
}

// AlterTableStatement builds an ALTER TABLE statement. Several changes can be
// gathered in a single statement, so ClickHouse applies them at once.
type AlterTableStatement struct {
	statement
	alter *parser.AlterTable
	// settings is the MODIFY SETTING clause, which holds every setting at
	// once. ClickHouse does not accept it more than once in a statement.
	settings *parser.AlterTableModifySetting
}

// AlterTable starts an ALTER TABLE statement with no change yet.
func AlterTable(name TableName) *AlterTableStatement {
	alter := &parser.AlterTable{TableIdentifier: name.node()}
	return &AlterTableStatement{node: alter, alter: alter}
}

// Len returns how many changes the statement carries.
func (s *AlterTableStatement) Len() int {
	return len(s.alter.AlterExprs)
}

func (s *AlterTableStatement) add(clause parser.AlterTableClause) *AlterTableStatement {
	s.alter.AlterExprs = append(s.alter.AlterExprs, clause)
	return s
}

// AddColumn adds a column after the one named. An empty name adds it first.
func (s *AlterTableStatement) AddColumn(def ColumnDef, after string) *AlterTableStatement {
	clause := &parser.AlterTableAddColumn{Column: def.node}
	if after != "" {
		clause.After = &parser.NestedIdentifier{Ident: ident(after)}
	}
	return s.add(clause)
}

// ModifyColumn changes the definition of an existing column.
func (s *AlterTableStatement) ModifyColumn(def ColumnDef) *AlterTableStatement {
	return s.add(&parser.AlterTableModifyColumn{Column: def.node})
}

// DropColumn removes a column.
func (s *AlterTableStatement) DropColumn(name string) *AlterTableStatement {
	return s.add(&parser.AlterTableDropColumn{
		ColumnName: &parser.NestedIdentifier{Ident: ident(name)},
	})
}

// ModifyOrderBy changes the sorting key of the table.
func (s *AlterTableStatement) ModifyOrderBy(exprs ...Expr) *AlterTableStatement {
	return s.add(&parser.AlterTableModifyOrderBy{OrderBy: Tuple(exprs...).node})
}

// ModifyTTL changes for how long the rows are kept.
func (s *AlterTableStatement) ModifyTTL(expr Expr) *AlterTableStatement {
	return s.add(&parser.AlterTableModifyTTL{
		TTL: &parser.TTLClause{Items: []*parser.TTLExpr{{Expr: expr.node}}},
	})
}

// ModifySetting changes one setting of the table. All the settings end up in a
// single clause.
func (s *AlterTableStatement) ModifySetting(name string, value Expr) *AlterTableStatement {
	if s.settings == nil {
		s.settings = &parser.AlterTableModifySetting{}
		s.add(s.settings)
	}
	s.settings.Settings = append(s.settings.Settings,
		&parser.SettingExpr{Name: ident(name), Expr: value.node})
	return s
}

// AddIndex adds a skip index on a column.
func (s *AlterTableStatement) AddIndex(name string, column, indexType Expr, granularity uint64) *AlterTableStatement {
	return s.add(&parser.AlterTableAddIndex{Index: &parser.TableIndex{
		Name:        &parser.NestedIdentifier{Ident: ident(name)},
		ColumnExpr:  &parser.ColumnExpr{Expr: column.node},
		ColumnType:  indexType.node,
		Granularity: &parser.NumberLiteral{Literal: fmt.Sprintf("%d", granularity)},
	}})
}

// DropIndex removes a skip index.
func (s *AlterTableStatement) DropIndex(name string) *AlterTableStatement {
	return s.add(&parser.AlterTableDropIndex{
		IndexName: &parser.NestedIdentifier{Ident: ident(name)},
	})
}

// DropTableStatement builds a DROP TABLE statement.
type DropTableStatement struct {
	statement
}

// DropTable builds a "DROP TABLE IF EXISTS … SYNC" statement. It also works on
// a view. SYNC makes ClickHouse wait for the table to be really gone, so it can
// be created again right after.
func DropTable(name TableName) DropTableStatement {
	return DropTableStatement{statement{node: &parser.DropStmt{
		DropTarget: "TABLE",
		Name:       name.node(),
		IfExists:   true,
		Modifier:   "SYNC",
	}}}
}

// CreateDatabase builds a "CREATE DATABASE IF NOT EXISTS" statement.
func CreateDatabase(name string) Statement {
	return statement{node: &parser.CreateDatabase{
		Name:        ident(name),
		IfNotExists: true,
	}}
}

// DropDatabase builds a "DROP DATABASE IF EXISTS … SYNC" statement. SYNC makes
// ClickHouse wait for the database to be really gone, so it can be created
// again right after.
func DropDatabase(name string) Statement {
	return statement{node: &parser.DropDatabase{
		Name:     ident(name),
		IfExists: true,
		Modifier: "SYNC",
	}}
}

// SystemReloadDictionary builds a "SYSTEM RELOAD DICTIONARY" statement.
func SystemReloadDictionary(name TableName) Statement {
	return statement{node: &parser.SystemStmt{Expr: &parser.SystemReloadExpr{
		Type:       "DICTIONARY",
		Dictionary: name.node(),
	}}}
}
