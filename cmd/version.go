// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package cmd

import (
	"net/http"
	"runtime"
	runtimedebug "runtime/debug"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"akvorado/common/helpers"
	"akvorado/common/reporter"
	"akvorado/common/schema"
)

func init() {
	RootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  `Display version and build information about akvorado.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.Printf("akvorado %s\n", helpers.AkvoradoVersion)
		cmd.Printf("  Built with: %s\n", runtime.Version())
		if info, ok := runtimedebug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if strings.HasPrefix(setting.Key, "GO") {
					cmd.Printf("  Build setting %s=%s\n", setting.Key, setting.Value)
				}
			}
		}
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
		"version":  helpers.AkvoradoVersion,
		"compiler": runtime.Version(),
	})
}

func versionMetrics(r *reporter.Reporter) {
	r.GaugeVec(reporter.GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "compiler"}).
		WithLabelValues(helpers.AkvoradoVersion, runtime.Version()).Set(1)
}
