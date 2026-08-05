// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"context"

	sb "akvorado/common/sqlbuilder"
)

// ExecOnCluster executes a statement. When a cluster is configured, the
// statement gets an ON CLUSTER clause first, so ClickHouse runs it on every
// server of the cluster.
func (c *Component) ExecOnCluster(ctx context.Context, statement sb.Statement, args ...any) error {
	return c.Exec(ctx, onClusterSQL(statement, c.config.Cluster), args...)
}

// onClusterSQL renders a statement as SQL, adding the ON CLUSTER clause when a
// cluster is provided.
func onClusterSQL(statement sb.Statement, cluster string) string {
	if cluster == "" {
		return statement.String()
	}
	return statement.OnCluster(cluster).String()
}
