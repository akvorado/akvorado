package cmd

import (
	"runtime"

	"github.com/spf13/cobra"
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
