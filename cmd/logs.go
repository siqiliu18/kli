package cmd

import (
	"fmt"

	"github.com/siqiliu/kli/internal/k8s"
	"github.com/spf13/cobra"
)

// followLogs is bound to --follow / -f. When true, the log stream stays open.
var followLogs bool

// grepPattern is bound to --grep. When non-empty, only matching lines are printed.
var grepPattern string

// container is bound to --container / -c. Required for multi-container pods.
var container string

func init() {
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Stream logs continuously (tail -f style)")
	logsCmd.Flags().StringVar(&grepPattern, "grep", "", "Filter output to lines containing this string")
	logsCmd.Flags().StringVarP(&container, "container", "c", "", "Container name (required for multi-container pods)")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs <pod_name>",
	Short: "Stream pod logs",
	Long: `Stream logs from a pod with optional follow and grep filtering.

Examples:
  # Print current logs and exit
  kli logs <pod> -n <namespace>

  # Tail logs continuously (stays open until Ctrl+C)
  kli logs <pod> -n <namespace> --follow

  # Print only lines containing "error"
  kli logs <pod> -n <namespace> --grep error

  # Combine: tail and filter
  kli logs <pod> -n <namespace> --follow --grep error`,
	Args: cobra.ExactArgs(1), // guards against missing pod name — prevents args[0] panic
	RunE: func(cmd *cobra.Command, args []string) error {
		pod := args[0]
		client, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}
		if err := client.NamespaceExists(namespace); err != nil {
			return err
		}
		return client.Logs(pod, namespace, container, grepPattern, followLogs)
	},
}
