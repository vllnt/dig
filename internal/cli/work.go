package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/vllnt/dig/internal/kb"
	"github.com/vllnt/dig/internal/policy"
	"github.com/vllnt/dig/internal/store"
)

func newWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Manage isolated work views (worktree-like)",
		Long:  "A view is an isolated unit of work: a pointer to a base manifest plus\nproposed ops. Workers fill views in parallel; 'dig merge' folds them back —\ndisjoint changes automatically, overlaps as conflicts.",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create <name>",
		Short: "Open a new view on the current head",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, closeFn, err := openStore()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := st.CreateView(args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "view %q opened on %s (%s)\n", v.Name, v.Base, v.State)
			return nil
		},
	})

	var asJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List views and their states",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, closeFn, err := openStore()
			if err != nil {
				return err
			}
			defer closeFn()
			views, err := st.ListViews()
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(views)
			}
			if len(views) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no views — open one with 'dig work create <name>'")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			for _, v := range views {
				extra := ""
				if v.Conflict != "" {
					extra = v.Conflict
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\tbase %s\t%d op(s)\t%s\n", v.Name, v.State, v.Base, len(v.Ops), extra)
			}
			return tw.Flush()
		},
	}
	list.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "abort <name>",
		Short: "Discard a view (head and disk untouched)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, closeFn, err := openStore()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := st.AbortView(args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "view %q aborted\n", v.Name)
			return nil
		},
	})

	var mine, theirs bool
	resolve := &cobra.Command{
		Use:   "resolve <view>",
		Short: "Settle an ESCALATED view — the human decision",
		Long:  "--mine applies the held ops (your view wins); --theirs discards them\n(what's at head stands). Exactly one must be chosen.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if mine == theirs {
				return fmt.Errorf("choose exactly one of --mine or --theirs")
			}
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			accept := "theirs"
			if mine {
				accept = "mine"
			}
			v, m, err := st.ResolveView(k.Root, args[0], accept)
			if err != nil {
				return err
			}
			if m != nil {
				if err := rebuildIndex(k.Dig(), st, m, cmd.ErrOrStderr()); err != nil {
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "resolved %q (%s) → manifest %s\n", v.Name, accept, m.ID)
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "resolved %q (%s) → %s\n", v.Name, accept, v.State)
			return nil
		},
	}
	resolve.Flags().BoolVar(&mine, "mine", false, "apply the held ops — the view wins")
	resolve.Flags().BoolVar(&theirs, "theirs", false, "discard the held ops — head stands")
	cmd.AddCommand(resolve)
	return cmd
}

func newMergeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge <view>",
		Short: "Merge a view back via the escalation ladder",
		Long:  "Per op: untouched paths apply; compatible changes union (labels, follow\nmoves); rule precedence resolves attributed conflicts; the rest is held —\nthe view goes ESCALATED with only the conflicted ops, everything else lands.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := kb.Resolve(kbFlag)
			if err != nil {
				return err
			}
			st, err := store.Open(k.Dig())
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			v, m, err := st.MergeView(k.Root, args[0], ruleRank(k))
			if err != nil {
				return err
			}
			if m != nil {
				if err := rebuildIndex(k.Dig(), st, m, cmd.ErrOrStderr()); err != nil {
					return err
				}
			}
			out := cmd.OutOrStdout()
			switch v.State {
			case store.StateMerged:
				_, _ = fmt.Fprintf(out, "merged %q → manifest %s\n", v.Name, m.ID)
			case store.StateEscalated:
				if m != nil {
					_, _ = fmt.Fprintf(out, "partially merged %q → manifest %s\n", v.Name, m.ID)
				}
				_, _ = fmt.Fprintf(out, "ESCALATED: %s\nresolve with 'dig work resolve %s --mine' or '--theirs'\n", v.Conflict, v.Name)
			default:
				_, _ = fmt.Fprintf(out, "view %q → %s: %s\n", v.Name, v.State, v.Conflict)
			}
			return nil
		},
	}
}

// ruleRank builds the precedence function from the KB's policy: a rule's rank
// is its position in the policy file (earlier = stronger). No policy → nil.
func ruleRank(k kb.KB) store.RuleRank {
	pol, err := policy.Load(filepath.Join(k.Dig(), policy.File))
	if err != nil {
		return nil
	}
	pos := map[string]int{}
	for i, r := range pol.Rules {
		pos[r.Name] = i
	}
	return func(rule string) int {
		if i, ok := pos[rule]; ok {
			return i
		}
		return -1
	}
}

// openStore resolves the KB and opens its store — shared by the small view
// subcommands that don't need the root path.
func openStore() (*store.Store, func(), error) {
	k, err := kb.Resolve(kbFlag)
	if err != nil {
		return nil, nil, err
	}
	st, err := store.Open(k.Dig())
	if err != nil {
		return nil, nil, err
	}
	return st, func() { _ = st.Close() }, nil
}
