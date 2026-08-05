// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package sqlbuilder

import (
	"regexp"
	"strings"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
)

// bareIdentifier matches the identifiers ClickHouse writes without backticks.
// See https://clickhouse.com/docs/sql-reference/syntax#identifiers. This is not
// complete, as keywords also have to be quoted, but for our purpose, it's enough.
var bareIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

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
	return backquotedIdent(name)
}

// backquotedIdent builds an identifier always written between backticks, the
// way ClickHouse writes the columns of a table back.
func backquotedIdent(name string) *parser.Ident {
	return &parser.Ident{Name: escapeIdentifier(name), QuoteType: parser.BackTicks}
}
