// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// bareIdentifierRegexp tells which identifiers do not need to be quoted. See
// https://clickhouse.com/docs/sql-reference/syntax#identifiers for the official
// regex.
var bareIdentifierRegexp = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// QuoteIdentifier quotes a ClickHouse identifier (table name, column name,
// database name, cluster name, etc.) using backtick escaping. Identifiers that
// are already valid bare identifiers are returned unchanged. This is not 100%
// complete as keywords also needs to be quoted!
func QuoteIdentifier(name string) string {
	if bareIdentifierRegexp.MatchString(name) {
		return name
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// ExecOnCluster executes a query on a cluster. It's a wrapper around Exec()
// invoking TransformQueryOnCluster.
func (c *Component) ExecOnCluster(ctx context.Context, query string, args ...any) error {
	if c.config.Cluster != "" {
		query = TransformQueryOnCluster(query, c.config.Cluster)
	}
	return c.Exec(ctx, query, args...)
}

var (
	spacesRegexp                   = regexp.MustCompile(`\s+`)
	statementBeforeOnClusterRegexp = regexp.MustCompile(fmt.Sprintf("^((?i)%s)", strings.Join([]string{
		`ALTER TABLE \S+`,
		`ATTACH DICTIONARY \S+`,
		`(ATTACH|CREATE) DATABASE( IF NOT EXISTS)? \S+`,
		`(ATTACH|CREATE( OR REPLACE)?|REPLACE) DICTIONARY( IF NOT EXISTS)? \S+`,
		`(ATTACH|CREATE) LIVE VIEW (IF NOT EXISTS)? \S+`,
		`(ATTACH|CREATE) MATERIALIZED VIEW( IF NOT EXISTS)? \S+`,
		`(ATTACH|CREATE( OR REPLACE)?|REPLACE)( TEMPORARY)? TABLE( IF NOT EXISTS)? \S+`,
		`(DETACH|DROP) DATABASE( IF EXISTS)? \S+`,
		`(DETACH|DROP) (DICTIONARY|(TEMPORARY )?TABLE|VIEW)( IF EXISTS?) \S+`,
		`KILL MUTATION`,
		`OPTIMIZE TABLE \S+`,
		`RENAME TABLE \S+ TO \S+`, // this is incomplete
		`TRUNCATE( TEMPORARY)?( TABLE)?( IF EXISTS)? \S+`,
		// not part of the grammar
		`SYSTEM RELOAD DICTIONARIES`,
		`SYSTEM RELOAD DICTIONARY`,
	}, "|")))
)

// TransformQueryOnCluster turns a ClickHouse query into its equivalent to be
// run on a cluster by adding the ON CLUSTER directive.
func TransformQueryOnCluster(query, cluster string) string {
	// From utils/antlr/ClickHouseParser.g4:
	//
	// ALTER TABLE tableIdentifier clusterClause? alterTableClause (COMMA alterTableClause)*
	// ATTACH DICTIONARY tableIdentifier clusterClause?
	// (ATTACH | CREATE) DATABASE (IF NOT EXISTS)? databaseIdentifier clusterClause? engineExpr?
	// (ATTACH | CREATE (OR REPLACE)? | REPLACE) DICTIONARY (IF NOT EXISTS)? tableIdentifier uuidClause? clusterClause? dictionarySchemaClause dictionaryEngineClause
	// (ATTACH | CREATE) LIVE VIEW (IF NOT EXISTS)? tableIdentifier uuidClause? clusterClause? (WITH TIMEOUT DECIMAL_LITERAL?)? destinationClause? tableSchemaClause? subqueryClause
	// (ATTACH | CREATE) MATERIALIZED VIEW (IF NOT EXISTS)? tableIdentifier uuidClause? clusterClause? tableSchemaClause? (destinationClause | engineClause POPULATE?) subqueryClause
	// (ATTACH | CREATE (OR REPLACE)? | REPLACE) TEMPORARY? TABLE (IF NOT EXISTS)? tableIdentifier uuidClause? clusterClause? tableSchemaClause? engineClause? subqueryClause?
	// (ATTACH | CREATE) (OR REPLACE)? VIEW (IF NOT EXISTS)? tableIdentifier uuidClause? clusterClause? tableSchemaClause? subqueryClause
	// (DETACH | DROP) DATABASE (IF EXISTS)? databaseIdentifier clusterClause?
	// (DETACH | DROP) (DICTIONARY | TEMPORARY? TABLE | VIEW) (IF EXISTS)? tableIdentifier clusterClause? (NO DELAY)?
	// KILL MUTATION clusterClause? whereClause (SYNC | ASYNC | TEST)?
	// OPTIMIZE TABLE tableIdentifier clusterClause? partitionClause? FINAL? DEDUPLICATE?;
	// RENAME TABLE tableIdentifier TO tableIdentifier (COMMA tableIdentifier TO tableIdentifier)* clusterClause?;
	// TRUNCATE TEMPORARY? TABLE? (IF EXISTS)? tableIdentifier clusterClause?;

	// In ClickHouse, an identifier uses the following syntax:
	//
	// IDENTIFIER
	//     : (LETTER | UNDERSCORE) (LETTER | UNDERSCORE | DEC_DIGIT)*
	//     | BACKQUOTE ( ~([\\`]) | (BACKSLASH .) | (BACKQUOTE BACKQUOTE) )* BACKQUOTE
	//     | QUOTE_DOUBLE ( ~([\\"]) | (BACKSLASH .) | (QUOTE_DOUBLE QUOTE_DOUBLE) )* QUOTE_DOUBLE
	//     ;
	//
	// Since we don't have to accept everything, we simplify it to \S+.
	query = strings.TrimSpace(spacesRegexp.ReplaceAllString(query, " "))
	prefix := statementBeforeOnClusterRegexp.FindString(query)
	if prefix == "" {
		return query
	}

	return fmt.Sprintf("%s ON CLUSTER %s%s", prefix, QuoteIdentifier(cluster), query[len(prefix):])
}
