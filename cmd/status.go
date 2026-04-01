package cmd

import (
	"fmt"

	"github.com/siqiliu/kli/internal/k8s"
	"github.com/siqiliu/kli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show workload health and pod status per resource",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}
		if err := client.NamespaceExists(namespace); err != nil {
			return err
		}
		results, err := client.Status(namespace)
		if err != nil {
			return err
		}
		ui.PrintStatus(namespace, results)
		return nil
	},
}
