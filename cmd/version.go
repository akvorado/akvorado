// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"akvorado/common/clickhousedb"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

var (
	// Version contains the current version.
	Version = "dev"
)

func init() {
	RootCmd.AddCommand(versionCmd)
	clickhousedb.AkvoradoVersion = Version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  `Display version and build information about akvorado.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.Printf("akvorado %s\n", Version)
		cmd.Printf("  Built with: %s\n", runtime.Version())
		cmd.Println()

		sch, err := schema.New(schema.DefaultConfiguration())
		if err != nil {
			return err
		}
		cmd.Println("Can be disabled:")
		for k := schema.ColumnTimeReceived; k < schema.ColumnLast; k++ {
			column, ok := sch.LookupColumnByKey(k)
			if ok && !column.Disabled && !column.NoDisable && !slices.Contains(sch.ClickHousePrimaryKeys(), column.Name) {
				cmd.Printf("- %s\n", column.Name)
			}
		}
		cmd.Println()
		cmd.Println("Can be enabled:")
		for k := schema.ColumnTimeReceived; k < schema.ColumnLast; k++ {
			column, ok := sch.LookupColumnByKey(k)
			if ok && column.Disabled {
				cmd.Printf("- %s", column.Name)
				if column.ClickHouseMainOnly {
					cmd.Print(" (main table only)")
				}
				cmd.Println()
			}
		}
		return nil
	},
}

func versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":  Version,
		"compiler": runtime.Version(),
	})
}

func versionMetrics(r *reporter.Reporter) {
	r.GaugeVec(reporter.GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "compiler"}).
		WithLabelValues(Version, runtime.Version()).Set(1)
}
