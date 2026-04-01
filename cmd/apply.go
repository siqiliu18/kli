package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/siqiliu/kli/internal/k8s"
	"github.com/siqiliu/kli/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	applyCmd.Flags().StringP("file", "f", "", "File or folder to apply (required)")
	applyCmd.MarkFlagRequired("file")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without applying")
	rootCmd.AddCommand(applyCmd) // ← this is how subcommands are registered
}

// dryRun is bound to the --dry-run flag. When true, apply passes DryRunAll to
// the API server which validates and diffs without persisting changes.
var dryRun bool

var applyCmd = &cobra.Command{
	Use:   "apply -f <file|folder>",
	Short: "Apply Kubernetes resources with clean output",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Error is safe to ignore: MarkFlagRequired("file") guarantees the flag
		// is present and non-empty before RunE is called.
		file, _ := cmd.Flags().GetString("file")

		// If the user did not explicitly pass -n/--namespace, confirm before
		// deploying to the default namespace — a silent default risks applying
		// to the wrong environment.
		if !cmd.Flag("namespace").Changed {
			fmt.Printf("No namespace specified. Deploy to \"default\"? [y/N]: ")
			input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.ToLower(strings.TrimSpace(input)) != "y" {
				fmt.Println("Cancelled.")
				fmt.Println("  Run:  kubectl create namespace <name>")
				fmt.Printf("  Then: kli apply -f %s -n <name>\n", file)
				return nil
			}
		}

		client, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("failed to connect to cluster: %w", err)
		}
		if err := client.NamespaceExists(namespace); err != nil {
			return err
		}

		sp := ui.NewSpinner(fmt.Sprintf("Applying resources to %s...", namespace))
		sp.Start()
		results, err := client.Apply(file, namespace, dryRun)
		sp.Stop()

		if err != nil {
			return err
		}

		ui.PrintApplyResults(results)
		return nil
	},
}
