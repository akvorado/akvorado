// SPDX-FileCopyrightText: 2024 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package clickhousedb

import (
	"testing"

	"go.uber.org/mock/gomock"

	"akvorado/common/reporter"
	sb "akvorado/common/sqlbuilder"
)

func TestExecOnCluster(t *testing.T) {
	t.Run("without cluster", func(t *testing.T) {
		r := reporter.NewMock(t)
		c, mock := NewMock(t, r)
		mock.EXPECT().
			Exec(gomock.Any(), sb.SQLMatcher(t, "DROP TABLE IF EXISTS flows SYNC")).
			Return(nil)
		if err := c.ExecOnCluster(t.Context(), sb.DropTable(sb.Table("flows"))); err != nil {
			t.Fatalf("ExecOnCluster() error:\n%+v", err)
		}
	})

	t.Run("with cluster", func(t *testing.T) {
		r := reporter.NewMock(t)
		c, mock := NewMock(t, r)
		c.config.Cluster = "akvorado"
		mock.EXPECT().
			Exec(gomock.Any(), sb.SQLMatcher(t, "DROP TABLE IF EXISTS flows ON CLUSTER akvorado SYNC")).
			Return(nil)
		if err := c.ExecOnCluster(t.Context(), sb.DropTable(sb.Table("flows"))); err != nil {
			t.Fatalf("ExecOnCluster() error:\n%+v", err)
		}
	})
}
