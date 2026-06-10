package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bntvllnt/dig/internal/dedup"
	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/policy"
	"github.com/bntvllnt/dig/internal/store"
)

func newDedupCmd() *cobra.Command {
	var dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Find duplicates and collapse them per policy",
		Long:  "Groups files with identical content, keeps the canonical copy per the\n[dedup] policy (default keep-oldest), removes the rest. Ties escalate.\nContent survives in the store — 'dig undo' restores every copy.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			// Policy file is optional for dedup — defaults apply.
			var dpol policy.DedupPolicy
			if pol, err := policy.Load(filepath.Join(k.Dig(), policy.File)); err == nil {
				dpol = pol.Dedup
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			head, err := st.Head()
			if err != nil {
				return err
			}
			if head == nil {
				return fmt.Errorf("no manifest yet — run 'dig scan' first")
			}
			plan, err := dedup.BuildPlan(head, dpol)
			if err != nil {
				return err
			}

			if asJSON {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(plan); err != nil {
					return err
				}
			} else {
				renderDedupPlan(cmd, plan, dryRun)
			}
			if dryRun || plan.Empty() {
				return nil
			}

			m, err := dedup.Apply(k.Root, st, head, plan)
			if err != nil {
				return err
			}
			if err := rebuildIndex(k.Dig(), st, m); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed %d duplicate(s) → manifest %s (undo with 'dig undo')\n", plan.Removals(), m.ID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show duplicate sets without removing anything")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the plan as JSON")
	return cmd
}

func renderDedupPlan(cmd *cobra.Command, plan *dedup.Plan, dryRun bool) {
	out := cmd.OutOrStdout()
	if plan.Empty() && len(plan.Conflicts) == 0 {
		_, _ = fmt.Fprintln(out, "no duplicates — every file is unique")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	for _, s := range plan.Sets {
		_, _ = fmt.Fprintf(tw, "KEEP\t%s\tremove: %s\n", s.Keep, strings.Join(s.Remove, ", "))
	}
	for _, c := range plan.Conflicts {
		_, _ = fmt.Fprintf(tw, "CONFLICT\t%s\t%s\n", strings.Join(c.Paths, ", "), c.Reason)
	}
	_ = tw.Flush()
	if dryRun {
		_, _ = fmt.Fprintf(out, "[dry-run] %d set(s), %d removal(s), %d conflict(s) — nothing changed\n",
			len(plan.Sets), plan.Removals(), len(plan.Conflicts))
	}
}
