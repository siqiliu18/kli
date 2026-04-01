package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var namespace string

func init() {
	// Global flag available on all subcommands
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
}

var rootCmd = &cobra.Command{
	Use:          "kli",
	Short:        "A developer-friendly Kubernetes CLI",
	Long:         `kli wraps kubectl with clean UX: spinners, color-coded output, and readable logs.`,
	// SilenceUsage prevents cobra from printing the usage block on runtime
	// errors (e.g. cluster unreachable, image pull failure). Usage should
	// only appear for user input errors like missing flags or wrong arguments.
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
