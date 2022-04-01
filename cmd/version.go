package cmd

import (
	"encoding/json"
	"net/http"
	"runtime"

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

func versionHandler() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			versionInfo := struct {
				Version   string `json:"version"`
				BuildDate string `json:"build_date"`
				Compiler  string `json:"compiler"`
			}{
				Version:   Version,
				BuildDate: BuildDate,
				Compiler:  runtime.Version(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(versionInfo)
		})
}

func versionMetrics(r *reporter.Reporter) {
	r.GaugeVec(reporter.GaugeOpts{
		Name: "info",
		Help: "Akvorado build information",
	}, []string{"version", "build_date", "compiler"}).
		WithLabelValues(Version, BuildDate, runtime.Version()).Set(1)
}
