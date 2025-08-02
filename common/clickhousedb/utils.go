// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

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
func TransformQueryOnCluster(query string, cluster string) string {
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

	return fmt.Sprintf("%s ON CLUSTER %s%s", prefix, cluster, query[len(prefix):])
}
