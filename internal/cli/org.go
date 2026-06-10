package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bntvllnt/dig/internal/index"
	"github.com/bntvllnt/dig/internal/kb"
	"github.com/bntvllnt/dig/internal/organize"
	"github.com/bntvllnt/dig/internal/policy"
	"github.com/bntvllnt/dig/internal/store"
)

func newOrgCmd() *cobra.Command {
	var dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Apply organization policy (move / rename / label)",
		Long:  "Evaluates .dig/policy.toml over the KB and applies the resulting changeset.\nEvery change is journaled and reversible with 'dig undo'.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			pol, err := policy.Load(filepath.Join(k.Dig(), policy.File))
			if err != nil {
				return fmt.Errorf("policy: %w", err)
			}
			rules, err := pol.Compile()
			if err != nil {
				return err
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
			plan, err := organize.BuildPlan(k.Root, head, rules)
			if err != nil {
				return err
			}

			if asJSON {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(plan); err != nil {
					return err
				}
			} else {
				renderPlan(cmd, plan, dryRun)
			}
			if dryRun || plan.Empty() && len(plan.Unsorted) == 0 {
				return nil
			}

			m, err := organize.Apply(k.Root, st, head, plan)
			if err != nil {
				return err
			}
			idx, err := index.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()
			if err := idx.Rebuild(m); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Applied %d op(s) → manifest %s (undo with 'dig undo')\n", len(plan.Ops), m.ID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the plan without changing anything")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the plan as JSON")
	return cmd
}

func renderPlan(cmd *cobra.Command, plan *organize.Plan, dryRun bool) {
	out := cmd.OutOrStdout()
	if plan.Empty() && len(plan.Conflicts) == 0 && len(plan.Unsorted) == 0 {
		_, _ = fmt.Fprintln(out, "nothing to do — KB matches policy")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	for _, op := range plan.Ops {
		switch op.Kind {
		case organize.OpMove:
			_, _ = fmt.Fprintf(tw, "MOVE\t%s\t→ %s\t(%s)\n", op.From, op.To, op.Rule)
		case organize.OpLabel:
			_, _ = fmt.Fprintf(tw, "LABEL\t%s\t+%s\t(%s)\n", op.From, strings.Join(op.Labels, ","), op.Rule)
		}
	}
	for _, op := range plan.Pinned {
		_, _ = fmt.Fprintf(tw, "PINNED\t%s\twould → %s\t(%s) — human-placed; remove %s label to re-apply policy\n",
			op.From, op.To, op.Rule, policy.PinnedLabel)
	}
	for _, c := range plan.Conflicts {
		_, _ = fmt.Fprintf(tw, "CONFLICT\t%s\t%s\t(%s)\n", c.Path, c.Reason, c.Rule)
	}
	for _, p := range plan.Unsorted {
		_, _ = fmt.Fprintf(tw, "UNSORTED\t%s\t+%s\t\n", p, policy.UnsortedLabel)
	}
	_ = tw.Flush()
	if dryRun {
		_, _ = fmt.Fprintf(out, "[dry-run] %d op(s), %d conflict(s), %d unsorted — nothing changed\n",
			len(plan.Ops), len(plan.Conflicts), len(plan.Unsorted))
	}
}

func newPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Inspect and validate the organization policy",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Parse + validate .dig/policy.toml, explain each rule",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			pol, err := policy.Load(filepath.Join(k.Dig(), policy.File))
			if err != nil {
				return fmt.Errorf("policy invalid: %w", err)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, r := range pol.Rules {
				var does []string
				if r.Into != "" {
					does = append(does, "into "+r.Into)
				}
				if r.Rename != "" {
					does = append(does, "rename "+r.Rename)
				}
				if len(r.Label) > 0 {
					does = append(does, "label "+strings.Join(r.Label, ","))
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\n", r.Name, strings.Join(does, " · "))
			}
			_ = tw.Flush()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "policy OK — %d rule(s)\n", len(pol.Rules))
			return nil
		},
	})
	return cmd
}
