package cmd

import (
	"fmt"

	"github.com/siqiliu/kli/internal/k8s"
	"github.com/siqiliu/kli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	undeployCmd.Flags().StringP("file", "f", "", "File or folder to undeploy (required)")
	undeployCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(undeployCmd)
}

var undeployCmd = &cobra.Command{
	Use:   "undeploy -f <file|folder>",
	Short: "Delete resources with per-resource results",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Error is safe to ignore: MarkFlagRequired("file") guarantees the flag
		// is present and non-empty before RunE is called.
		file, _ := cmd.Flags().GetString("file")
		client, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}
		if err := client.NamespaceExists(namespace); err != nil {
			return err
		}

		fmt.Printf("Namespace: %s\n\n", ui.Green.Render(namespace))

		sp := ui.NewSpinner(fmt.Sprintf("Undeploying resources from %s...", namespace))
		sp.Start()
		results, err := client.Undeploy(file, namespace)
		sp.Stop()

		if err != nil {
			return err
		}

		ui.PrintUndeployResults(results)
		return nil
	},
}
