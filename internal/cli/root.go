// Package cli wires dig's cobra command tree. Commands are thin: they resolve
// the target KB, call into internal packages, and render output. Read commands
// support --json; mutating commands support --dry-run.
package cli

import (
	"github.com/spf13/cobra"
)

// kbFlag holds the --kb value, shared across commands.
var kbFlag string

// NewRoot builds the root command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "dig",
		Short:         "A librarian for your knowledge base",
		Long:          "dig keeps a knowledge base organized to your policy — find, organize, version, reconcile — safely and reversibly.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&kbFlag, "kb", "", "KB name or path (default: the KB at or above the current directory)")

	root.AddCommand(
		newInitCmd(),
		newScanCmd(),
		newFindCmd(),
		newExportCmd(),
		newOrgCmd(),
		newDedupCmd(),
		newDriftCmd(),
		newReconcileCmd(),
		newPolicyCmd(),
		newLogCmd(),
		newUndoCmd(),
	)
	return root
}
