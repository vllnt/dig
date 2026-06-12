// Package cli wires dig's cobra command tree. Commands are thin: they resolve
// the target KB, call into internal packages, and render output. Read commands
// support --json; mutating commands support --dry-run.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// kbFlag holds the --kb value, shared across commands.
var kbFlag string

// BuildInfo carries the binary's build metadata for `dig --version`.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// Version is the binary's build metadata, set by main from ldflags. Defaults
// keep `go build`/`go install` working without GoReleaser.
var Version = BuildInfo{Version: "dev", Commit: "none", Date: "unknown"}

// NewRoot builds the root command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "dig",
		Short:         "A librarian for your knowledge base",
		Long:          "dig keeps a knowledge base organized to your policy — find, organize, version, reconcile — safely and reversibly.",
		Version:       fmt.Sprintf("%s (commit %s, built %s)", Version.Version, Version.Commit, Version.Date),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&kbFlag, "kb", "", "KB name or path (default: the KB at or above the current directory)")

	root.AddCommand(
		newInitCmd(),
		newScanCmd(),
		newFindCmd(),
		newEmbedCmd(),
		newMcpCmd(),
		newExportCmd(),
		newOrgCmd(),
		newDedupCmd(),
		newDriftCmd(),
		newReconcileCmd(),
		newWatchCmd(),
		newPolicyCmd(),
		newWorkCmd(),
		newMergeCmd(),
		newLogCmd(),
		newUndoCmd(),
	)
	return root
}
