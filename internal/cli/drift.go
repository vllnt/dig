package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/drift"
	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
	"github.com/vllnt/dig/internal/watch"
)

// loadRules loads + compiles the KB policy. Missing policy file is fine for
// drift/reconcile — external-edit absorption and dedup still work; policy
// ops are simply empty.
func loadRules(k kb.KB) ([]policy.CompiledRule, policy.DedupPolicy, error) {
	pol, err := policy.Load(filepath.Join(k.Dig(), policy.File))
	if errors.Is(err, os.ErrNotExist) {
		return nil, policy.DedupPolicy{}, nil
	}
	if err != nil {
		return nil, policy.DedupPolicy{}, fmt.Errorf("policy: %w", err)
	}
	rules, err := pol.Compile()
	if err != nil {
		return nil, policy.DedupPolicy{}, err
	}
	return rules, pol.Dedup, nil
}

func newDriftCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Report how the KB diverges from policy (read-only)",
		Long:  "Compares desired state (policy) with actual state (disk): external edits\nsince the last manifest, policy violations, unsorted files, duplicates.\nChanges nothing — 'dig reconcile' converges.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			rules, dpol, err := loadRules(k)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			rep, err := drift.BuildReport(k, st, rules, dpol)
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(rep)
			}
			out := cmd.OutOrStdout()
			if rep.Clean() {
				_, _ = fmt.Fprintln(out, "no drift — KB matches policy")
				return nil
			}
			tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
			for _, c := range rep.Changes {
				if c.Kind == drift.Renamed {
					_, _ = fmt.Fprintf(tw, "EDIT\t%s\t%s → %s\n", c.Kind, c.From, c.Path)
				} else {
					_, _ = fmt.Fprintf(tw, "EDIT\t%s\t%s\n", c.Kind, c.Path)
				}
			}
			for _, op := range rep.PolicyOps {
				_, _ = fmt.Fprintf(tw, "POLICY\t%s\t%s → %s\t(%s)\n", op.Kind, op.From, op.To, op.Rule)
			}
			for _, op := range rep.Pinned {
				_, _ = fmt.Fprintf(tw, "PINNED\t%s\twould → %s\t(%s)\n", op.From, op.To, op.Rule)
			}
			for _, c := range rep.Conflicts {
				_, _ = fmt.Fprintf(tw, "CONFLICT\t%s\t%s\n", c.Path, c.Reason)
			}
			for _, p := range rep.Unsorted {
				_, _ = fmt.Fprintf(tw, "UNSORTED\t%s\t\n", p)
			}
			for _, s := range rep.DupSets {
				_, _ = fmt.Fprintf(tw, "DUP\t%s\tremove: %s\n", s.Keep, strings.Join(s.Remove, ", "))
			}
			for _, c := range rep.DupConflicts {
				_, _ = fmt.Fprintf(tw, "DUP-TIE\t%s\t%s\n", strings.Join(c.Paths, ", "), c.Reason)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the report as JSON")
	return cmd
}

func newReconcileCmd() *cobra.Command {
	var dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Converge the KB back to policy — one shot",
		Long: "Absorbs external edits into history, applies policy (auto where safe),\n" +
			"collapses unambiguous duplicates. A path a human just moved is never\n" +
			"moved back automatically — policy disagreements on it are ESCALATED.\n" +
			"Each step is a separate journaled commit; 'dig undo' unwinds them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			rules, dpol, err := loadRules(k)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			sum, err := drift.Reconcile(k, st, rules, dpol, dryRun, drift.ModeOneShot)
			if err != nil {
				return err
			}
			if !dryRun {
				head, err := st.Head()
				if err != nil {
					return err
				}
				if err := rebuildIndex(k.Dig(), st, head); err != nil {
					return err
				}
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(sum)
			}
			renderSummary(cmd, sum, dryRun)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "compute the convergence without committing anything")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the summary as JSON")
	return cmd
}

func renderSummary(cmd *cobra.Command, sum *drift.Summary, dryRun bool) {
	out := cmd.OutOrStdout()
	if sum.Empty() {
		_, _ = fmt.Fprintln(out, "already converged — nothing to reconcile")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	for _, c := range sum.Absorbed {
		if c.Kind == drift.Renamed {
			_, _ = fmt.Fprintf(tw, "ABSORB\t%s\t%s → %s\n", c.Kind, c.From, c.Path)
		} else {
			_, _ = fmt.Fprintf(tw, "ABSORB\t%s\t%s\n", c.Kind, c.Path)
		}
	}
	for _, op := range sum.Applied {
		_, _ = fmt.Fprintf(tw, "APPLY\t%s\t%s\t%s\t(%s)\n", op.Kind, op.From, watch.OpTarget(op.From, op.To, op.Labels), op.Rule)
	}
	for _, op := range sum.Proposed {
		_, _ = fmt.Fprintf(tw, "PROPOSE\t%s\twould %s\t(%s — rule autonomy; re-run to consent)\n", op.From, watch.OpTarget(op.From, op.To, op.Labels), op.Rule)
	}
	for _, op := range sum.Escalated {
		_, _ = fmt.Fprintf(tw, "ESCALATE\t%s\twould → %s\t(%s) — human moved it; not auto-applied\n", op.From, op.To, op.Rule)
	}
	for _, c := range sum.Conflicts {
		_, _ = fmt.Fprintf(tw, "CONFLICT\t%s\t%s\n", c.Path, c.Reason)
	}
	for _, s := range sum.Collapsed {
		_, _ = fmt.Fprintf(tw, "DEDUP\t%s\tremoved: %s\n", s.Keep, strings.Join(s.Remove, ", "))
	}
	for _, c := range sum.DupTies {
		_, _ = fmt.Fprintf(tw, "DUP-TIE\t%s\t%s\n", strings.Join(c.Paths, ", "), c.Reason)
	}
	_ = tw.Flush()
	if dryRun {
		_, _ = fmt.Fprintln(out, "[dry-run] nothing committed")
	} else {
		_, _ = fmt.Fprintf(out, "reconciled → head %s (each step undoable with 'dig undo')\n", sum.Head)
	}
}
