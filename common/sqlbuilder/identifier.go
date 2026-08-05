// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// bareIdentifier matches the identifiers ClickHouse writes without backticks.
// See https://clickhouse.com/docs/sql-reference/syntax#identifiers. This is not
// complete, as keywords also have to be quoted, but for our purpose, it's enough.
var bareIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// QuoteIdentifier quotes an identifier (table name, column name, database name,
// cluster name, etc.) the way ClickHouse writes it: with backticks only when it
// needs them. It is meant for the SQL not built with this package.
func QuoteIdentifier(name string) string {
	if bareIdentifier.MatchString(name) {
		return name
	}
	return fmt.Sprintf("`%s`", escapeIdentifier(name))
}

// escapeIdentifier doubles the backticks of a name, so it can be put between
// backticks.
func escapeIdentifier(name string) string {
	return strings.ReplaceAll(name, "`", "``")
}

// quoteType tells how ClickHouse writes an identifier: with backticks only when
// it has to.
func quoteType(name string) int {
	if bareIdentifier.MatchString(name) {
		return parser.Unquoted
	}
	return parser.BackTicks
}

// ident builds an identifier, backquoted only when it has to be. The formatter
// writes the name as is between the backticks, so it is escaped here.
func ident(name string) *parser.Ident {
	if bareIdentifier.MatchString(name) {
		return &parser.Ident{Name: name, QuoteType: parser.Unquoted}
	}
	return &parser.Ident{Name: escapeIdentifier(name), QuoteType: parser.BackTicks}
}
