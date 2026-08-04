// SPDX-FileCopyrightText: 2026 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package filter

import (
	"fmt"
	"testing"

	sb "akvorado/common/sqlbuilder"
)

// checkWhereParses checks an expression is a valid boolean expression. The
// grammar builds a syntax tree, so this checks the tree turns back into SQL a
// database would accept.
func checkWhereParses(t *testing.T, where string) {
	t.Helper()
	if where == "" {
		return
	}
	sb.CheckStatement(t, fmt.Sprintf("SELECT 1 FROM flows WHERE %s", where))
}
