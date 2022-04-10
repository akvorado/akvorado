package cmd

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"akvorado/common/reporter"
)

var (
	// Version contains the current version.
	Version = "dev"
	// BuildDate contains a string with the build date.
	BuildDate = "unknown"
)

func init() {
	RootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long:  `Display version and build information about akvorado.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("akvorado %s\n", Version)
		cmd.Printf("  Build date: %s\n", BuildDate)
		cmd.Printf("  Built with: %s\n", runtime.Version())
	},
}

func versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    Version,
		"build-date": BuildDate,
		"compiler":   runtime.Version(),
	})
}

func versionMetrics(r *reporter.Reporter) {
	r.GaugeVec(reporter.GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "build_date", "compiler"}).
		WithLabelValues(Version, BuildDate, runtime.Version()).Set(1)
}
